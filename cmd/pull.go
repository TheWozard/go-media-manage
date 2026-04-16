package cmd

import (
	"fmt"
	"os"
	"path/filepath"

	"github.com/spf13/cobra"
	"go-media-manage/internal/cache"
	"go-media-manage/internal/config"
	"go-media-manage/internal/images"
	"go-media-manage/internal/nfo"
	"go-media-manage/internal/scanner"
	"go-media-manage/internal/scope"
	"go-media-manage/internal/tmdb"
)

var (
	flagPullImages    bool
	flagPullAllImages bool
	flagPullMetadata  bool
	flagPullRoot      bool
)

var pullCmd = &cobra.Command{
	Use:   "pull <directory>",
	Short: "Download metadata and artwork using the cached match",
	Long: `Reads matches.json from the directory and fetches data from TMDB.

Scope is inferred from the directory name: a "Season N" directory targets
that season only; any other directory targets everything. Use --root to
restrict to show-level files only (tvshow.nfo, poster, fanart).

Pass --images and/or --metadata to select what to fetch.`,
	Args: cobra.ExactArgs(1),
	RunE: runPull,
}

func init() {
	pullCmd.Flags().BoolVar(&flagPullImages, "images", false, "Download missing artwork")
	pullCmd.Flags().BoolVar(&flagPullAllImages, "all-images", false, "Download all artwork, replacing existing files")
	pullCmd.Flags().BoolVar(&flagPullMetadata, "metadata", false, "Write NFO files")
	pullCmd.Flags().BoolVar(&flagPullRoot, "root", false, "Restrict to show-level files only")
	rootCmd.AddCommand(pullCmd)
}


func runPull(cmd *cobra.Command, args []string) error {
	if !flagPullImages && !flagPullAllImages && !flagPullMetadata {
		return fmt.Errorf("pass --images and/or --metadata to specify what to fetch")
	}

	dir, err := filepath.Abs(args[0])
	if err != nil {
		return err
	}

	var sc scope.Scope
	if flagPullRoot {
		sc = scope.Root()
	} else {
		sc = scope.FromDir(dir)
	}

	cfg, err := config.Load()
	if err != nil {
		return err
	}
	if err := cfg.Validate(); err != nil {
		return err
	}

	c, err := cache.New(sc.RootDir(dir))
	if err != nil {
		return err
	}

	entry, ok := c.Get()
	if !ok {
		return fmt.Errorf("no match cached for %s — run 'match' first", dir)
	}

	client := tmdb.NewClient(cfg.TMDBToken, cfg.Language)

	switch entry.MediaType {
	case "tv":
		result, err := scanner.Scan(dir, scanner.TypeTV)
		if err != nil {
			return err
		}
		return pullTV(dir, sc, result, entry, client)
	case "movie":
		if sc.IsSeasonScope() {
			return fmt.Errorf("season scope is not valid for movies — use all or root")
		}
		return pullMovie(dir, entry, client)
	case "list":
		if sc.IsSeasonScope() {
			return fmt.Errorf("season scope is not valid for lists — use the show root directory")
		}
		result, err := scanner.Scan(dir, scanner.TypeMovie)
		if err != nil {
			return err
		}
		return pullList(dir, result, entry, client)
	default:
		return fmt.Errorf("unknown media type %q in cache", entry.MediaType)
	}
}

// ─── TV ──────────────────────────────────────────────────────────────────────

func pullTV(dir string, sc scope.Scope, result *scanner.ScanResult, entry *cache.Entry, client *tmdb.Client) error {
	rootDir := sc.RootDir(dir)

	detail, err := client.GetTVShow(entry.TMDBID)
	if err != nil {
		return fmt.Errorf("fetching show details: %w", err)
	}

	fmt.Printf("Pulling: %s (%s)\n", detail.Name, detail.FirstAirDate[:minInt(4, len(detail.FirstAirDate))])

	// ── Show-level ───────────────────────────────────────────────────────────
	if sc.IncludesRoot() {
		if flagPullMetadata {
			fmt.Println("Writing tvshow.nfo …")
			if err := nfo.WriteTVShow(rootDir, detail); err != nil {
				return err
			}
		}
		if flagPullImages || flagPullAllImages {
			fmt.Println("Downloading show artwork …")
			if err := images.DownloadTVShow(rootDir, detail, flagPullAllImages); err != nil {
				fmt.Fprintf(os.Stderr, "warning: %v\n", err)
			}
		}
	}

	// ── Season / episode operations ──────────────────────────────────────────
	inScope := sc.Seasons(result.Files)
	if len(inScope) == 0 {
		fmt.Println("Done.")
		return nil
	}

	// Season posters live in the show root dir.
	if flagPullImages || flagPullAllImages {
		var subset []tmdb.SeasonSummary
		for _, s := range detail.Seasons {
			if sc.IncludesSeason(s.SeasonNumber) {
				subset = append(subset, s)
			}
		}
		if len(subset) > 0 {
			fmt.Println("Downloading season poster(s) …")
			if err := images.DownloadSeasonPosters(rootDir, subset, flagPullAllImages); err != nil {
				fmt.Fprintf(os.Stderr, "warning: %v\n", err)
			}
		}
	}

	bySeasonDir := scanner.GroupBySeasonDir(result.Files)

	// Build episode group map if one was selected.
	var groupMap map[[2]int]*tmdb.Episode
	if entry.EpisodeGroupID != "" {
		fmt.Printf("Fetching episode group %s …\n", entry.EpisodeGroupID)
		group, err := client.GetEpisodeGroup(entry.EpisodeGroupID)
		if err != nil {
			fmt.Fprintf(os.Stderr, "warning: could not fetch episode group: %v\n", err)
		} else {
			groupMap = tmdb.BuildGroupMap(group)
			fmt.Printf("Using episode group: %s\n", group.Name)
		}
	}

	for _, seasonNum := range inScope {
		if groupMap != nil {
			// ── Episode group path ───────────────────────────────────────────
			if flagPullMetadata {
				for seasonDir, filesInDir := range bySeasonDir {
					if len(filesInDir) == 0 || filesInDir[0].Season != seasonNum {
						continue
					}
					fakeSeason := &tmdb.Season{SeasonNumber: seasonNum, Name: fmt.Sprintf("Season %d", seasonNum)}
					fmt.Printf("  Writing season.nfo → %s\n", seasonDir)
					if err := nfo.WriteSeason(seasonDir, fakeSeason, entry.TMDBID); err != nil {
						fmt.Fprintf(os.Stderr, "  warning: %v\n", err)
					}
				}
			}
			for _, f := range result.Files {
				if f.Season != seasonNum {
					continue
				}
				ep, ok := groupMap[[2]int{f.Season, f.Episode}]
				if !ok {
					fmt.Fprintf(os.Stderr, "  warning: S%02dE%02d not found in episode group\n", f.Season, f.Episode)
					continue
				}
				fmt.Printf("  S%02dE%02d %s\n", f.Season, f.Episode, ep.Name)
				if flagPullMetadata {
					if err := nfo.WriteEpisode(f.Path, ep, entry.TMDBID); err != nil {
						fmt.Fprintf(os.Stderr, "  warning: nfo: %v\n", err)
					}
				}
				if flagPullImages || flagPullAllImages {
					if err := images.DownloadEpisodeThumb(f.Path, ep, flagPullAllImages); err != nil {
						fmt.Fprintf(os.Stderr, "  warning: thumb: %v\n", err)
					}
				}
			}
		} else {
			// ── Standard path ────────────────────────────────────────────────
			fmt.Printf("Fetching season %d …\n", seasonNum)
			season, err := client.GetSeason(entry.TMDBID, seasonNum)
			if err != nil {
				fmt.Fprintf(os.Stderr, "warning: season %d: %v\n", seasonNum, err)
				continue
			}

			epMap := make(map[int]*tmdb.Episode)
			for i := range season.Episodes {
				epMap[season.Episodes[i].EpisodeNumber] = &season.Episodes[i]
			}

			if flagPullMetadata {
				for seasonDir, filesInDir := range bySeasonDir {
					if len(filesInDir) == 0 || filesInDir[0].Season != seasonNum {
						continue
					}
					fmt.Printf("  Writing season.nfo → %s\n", seasonDir)
					if err := nfo.WriteSeason(seasonDir, season, entry.TMDBID); err != nil {
						fmt.Fprintf(os.Stderr, "  warning: %v\n", err)
					}
				}
			}

			for _, f := range result.Files {
				if f.Season != seasonNum {
					continue
				}
				ep, ok := epMap[f.Episode]
				if !ok {
					fmt.Fprintf(os.Stderr, "  warning: S%02dE%02d not found on TMDB\n", f.Season, f.Episode)
					continue
				}
				fmt.Printf("  S%02dE%02d %s\n", ep.SeasonNumber, ep.EpisodeNumber, ep.Name)
				if flagPullMetadata {
					if err := nfo.WriteEpisode(f.Path, ep, entry.TMDBID); err != nil {
						fmt.Fprintf(os.Stderr, "  warning: nfo: %v\n", err)
					}
				}
				if flagPullImages || flagPullAllImages {
					if err := images.DownloadEpisodeThumb(f.Path, ep, flagPullAllImages); err != nil {
						fmt.Fprintf(os.Stderr, "  warning: thumb: %v\n", err)
					}
				}
			}
		}
	}

	fmt.Println("Done.")
	return nil
}

// ─── Movie ────────────────────────────────────────────────────────────────────

func pullMovie(dir string, entry *cache.Entry, client *tmdb.Client) error {
	detail, err := client.GetMovie(entry.TMDBID)
	if err != nil {
		return fmt.Errorf("fetching movie details: %w", err)
	}

	fmt.Printf("Pulling: %s (%s)\n", detail.Title, detail.ReleaseDate[:minInt(4, len(detail.ReleaseDate))])

	if flagPullMetadata {
		fmt.Println("Writing movie.nfo …")
		if err := nfo.WriteMovie(dir, detail); err != nil {
			return err
		}
	}
	if flagPullImages || flagPullAllImages {
		fmt.Println("Downloading artwork …")
		if err := images.DownloadMovie(dir, detail, flagPullAllImages); err != nil {
			fmt.Fprintf(os.Stderr, "warning: %v\n", err)
		}
	}

	fmt.Println("Done.")
	return nil
}

// ─── List ─────────────────────────────────────────────────────────────────────

func pullList(dir string, result *scanner.ScanResult, entry *cache.Entry, client *tmdb.Client) error {
	// The anchor movie provides show-root artwork and tvshow.nfo.
	movie, err := client.GetMovie(entry.TMDBID)
	if err != nil {
		return fmt.Errorf("fetching anchor movie: %w", err)
	}

	list, err := client.GetList(entry.ListID)
	if err != nil {
		return fmt.Errorf("fetching list: %w", err)
	}

	items := tmdb.SortListItems(list.Items)
	fmt.Printf("Pulling: %s — season 1 from list %q (%d items)\n", movie.Title, list.Name, len(items))

	if flagPullMetadata {
		fmt.Println("Writing tvshow.nfo …")
		if err := nfo.WriteTVShowFromMovie(dir, movie); err != nil {
			return err
		}
	}
	if flagPullImages || flagPullAllImages {
		fmt.Println("Downloading show artwork …")
		if err := images.DownloadMovie(dir, movie, flagPullAllImages); err != nil {
			fmt.Fprintf(os.Stderr, "warning: %v\n", err)
		}
	}

	// Sort video files by filename for deterministic positional matching.
	files := result.Files
	sortFilesByName(files)

	if len(files) != len(items) {
		fmt.Fprintf(os.Stderr, "warning: %d video file(s) but %d list item(s) — matching by position up to the smaller count\n",
			len(files), len(items))
	}

	limit := minInt(len(files), len(items))
	for i := 0; i < limit; i++ {
		f := files[i]
		item := &items[i]
		episode := i + 1
		fmt.Printf("  S01E%02d %s\n", episode, item.EffectiveTitle())
		if flagPullMetadata {
			if err := nfo.WriteEpisodeFromListItem(f.Path, item, 1, episode); err != nil {
				fmt.Fprintf(os.Stderr, "  warning: nfo: %v\n", err)
			}
		}
		if flagPullImages || flagPullAllImages {
			if err := images.DownloadListItemThumb(f.Path, item, flagPullAllImages); err != nil {
				fmt.Fprintf(os.Stderr, "  warning: thumb: %v\n", err)
			}
		}
	}

	fmt.Println("Done.")
	return nil
}

func sortFilesByName(files []*scanner.MediaFile) {
	for i := 1; i < len(files); i++ {
		for j := i; j > 0 && files[j].Base < files[j-1].Base; j-- {
			files[j], files[j-1] = files[j-1], files[j]
		}
	}
}

func minInt(a, b int) int {
	if a < b {
		return a
	}
	return b
}
