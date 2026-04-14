package cmd

import (
	"bufio"
	"fmt"
	"os"
	"path/filepath"
	"regexp"
	"sort"
	"strconv"
	"strings"

	"github.com/spf13/cobra"
	"go-media-manage/internal/cache"
	"go-media-manage/internal/config"
	"go-media-manage/internal/images"
	"go-media-manage/internal/nfo"
	"go-media-manage/internal/scanner"
	"go-media-manage/internal/tmdb"
)

var (
	flagType    string
	flagDryRun  bool
	flagForce   bool
	flagNoArt   bool
)

var scanCmd = &cobra.Command{
	Use:   "scan <directory>",
	Short: "Scan a directory and fetch metadata from TMDB",
	Args:  cobra.ExactArgs(1),
	RunE:  runScan,
}

func init() {
	scanCmd.Flags().StringVarP(&flagType, "type", "t", "auto", "Media type: auto, tv, movie")
	scanCmd.Flags().BoolVar(&flagDryRun, "dry-run", false, "Print actions without writing files")
	scanCmd.Flags().BoolVarP(&flagForce, "force", "f", false, "Re-fetch even if cached")
	scanCmd.Flags().BoolVar(&flagNoArt, "no-art", false, "Write NFO files only, skip image downloads")
	rootCmd.AddCommand(scanCmd)
}

func runScan(cmd *cobra.Command, args []string) error {
	dir, err := filepath.Abs(args[0])
	if err != nil {
		return err
	}

	cfg, err := config.Load()
	if err != nil {
		return err
	}
	if err := cfg.Validate(); err != nil {
		return err
	}

	hint := scanner.MediaType(flagType)
	if hint != scanner.TypeTV && hint != scanner.TypeMovie && hint != scanner.TypeAuto {
		return fmt.Errorf("--type must be auto, tv, or movie")
	}

	result, err := scanner.Scan(dir, hint)
	if err != nil {
		return err
	}

	fmt.Printf("Found %d video file(s) in %s\n", len(result.Files), dir)
	fmt.Printf("Detected type : %s\n", result.MediaType)
	fmt.Printf("Title         : %s\n", result.Title)

	client := tmdb.NewClient(cfg.TMDBToken, cfg.Language)
	c, err := cache.New(dir)
	if err != nil {
		return err
	}

	switch result.MediaType {
	case scanner.TypeTV:
		return processTVShow(dir, result, client, c)
	case scanner.TypeMovie:
		return processMovie(dir, result, client, c)
	default:
		return fmt.Errorf("unknown media type")
	}
}

// ─── TV ──────────────────────────────────────────────────────────────────────

func processTVShow(dir string, result *scanner.ScanResult, client *tmdb.Client, c *cache.Cache) error {
	var showID int
	var groupID string

	if entry, ok := c.Get(); ok && !flagForce {
		fmt.Printf("Cache hit: %s → TMDB %d\n", entry.Title, entry.TMDBID)
		showID = entry.TMDBID
		groupID = entry.EpisodeGroupID
	} else {
		fmt.Printf("Searching TMDB for TV show: %q\n", result.Title)
		shows, err := client.SearchTV(result.Title)
		if err != nil {
			return err
		}
		if len(shows) == 0 {
			return fmt.Errorf("no results for %q", result.Title)
		}

		chosen, err := pickTV(shows)
		if err != nil {
			return err
		}
		showID = chosen.ID

		// Offer episode groups if any exist
		groups, err := client.GetEpisodeGroups(showID)
		if err == nil && len(groups) > 0 {
			groupID, err = pickEpisodeGroup(groups)
			if err != nil {
				return err
			}
		}

		if !flagDryRun {
			if err := c.Set("tv", chosen.Name, chosen.ID, groupID); err != nil {
				fmt.Fprintf(os.Stderr, "warning: could not save to cache: %v\n", err)
			}
		}
	}

	if flagDryRun {
		fmt.Println("[dry-run] Would write tvshow.nfo and download artwork")
		for _, f := range result.Files {
			fmt.Printf("[dry-run] Would write %s.nfo and thumb\n", f.Base)
		}
		return nil
	}

	// Fetch full show detail
	detail, err := client.GetTVShow(showID)
	if err != nil {
		return fmt.Errorf("fetching show details: %w", err)
	}

	fmt.Printf("Matched: %s (%s)\n", detail.Name, detail.FirstAirDate[:min(4, len(detail.FirstAirDate))])

	// Write show NFO + images
	fmt.Println("Writing tvshow.nfo …")
	if err := nfo.WriteTVShow(dir, detail); err != nil {
		return err
	}
	if !flagNoArt {
		fmt.Println("Downloading show artwork …")
		if err := images.DownloadTVShow(dir, detail); err != nil {
			fmt.Fprintf(os.Stderr, "warning: %v\n", err)
		}
	}

	bySeasonDir := groupBySeasonDir(result.Files)
	seasons := uniqueSeasons(result.Files)

	// Build episode group map if one was selected
	var groupMap map[[2]int]*tmdb.Episode
	if groupID != "" {
		fmt.Printf("Fetching episode group %s …\n", groupID)
		group, err := client.GetEpisodeGroup(groupID)
		if err != nil {
			fmt.Fprintf(os.Stderr, "warning: could not fetch episode group: %v\n", err)
		} else {
			groupMap = buildGroupMap(group)
			fmt.Printf("Using episode group: %s\n", group.Name)
		}
	}

	for _, seasonNum := range seasons {
		if groupMap != nil {
			// Write a minimal season NFO from the group data; skip season poster
			for seasonDir := range bySeasonDir {
				filesInDir := bySeasonDir[seasonDir]
				if len(filesInDir) == 0 || filesInDir[0].Season != seasonNum {
					continue
				}
				fakeSeason := &tmdb.Season{SeasonNumber: seasonNum, Name: fmt.Sprintf("Season %d", seasonNum)}
				fmt.Printf("  Writing season.nfo → %s\n", seasonDir)
				if err := nfo.WriteSeason(seasonDir, fakeSeason, showID); err != nil {
					fmt.Fprintf(os.Stderr, "  warning: %v\n", err)
				}
			}
		} else {
			fmt.Printf("Fetching season %d …\n", seasonNum)
			season, err := client.GetSeason(showID, seasonNum)
			if err != nil {
				fmt.Fprintf(os.Stderr, "warning: season %d: %v\n", seasonNum, err)
				continue
			}

			// Episode lookup map for standard ordering
			epMap := make(map[int]*tmdb.Episode)
			for i := range season.Episodes {
				epMap[season.Episodes[i].EpisodeNumber] = &season.Episodes[i]
			}

			for seasonDir := range bySeasonDir {
				filesInDir := bySeasonDir[seasonDir]
				if len(filesInDir) == 0 || filesInDir[0].Season != seasonNum {
					continue
				}
				fmt.Printf("  Writing season.nfo → %s\n", seasonDir)
				if err := nfo.WriteSeason(seasonDir, season, showID); err != nil {
					fmt.Fprintf(os.Stderr, "  warning: %v\n", err)
				}
			}

			if !flagNoArt {
				if err := images.DownloadSeason(dir, season); err != nil {
					fmt.Fprintf(os.Stderr, "  warning: season %d poster: %v\n", seasonNum, err)
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
				if err := nfo.WriteEpisode(f.Path, ep, showID); err != nil {
					fmt.Fprintf(os.Stderr, "  warning: nfo: %v\n", err)
				}
				if !flagNoArt {
					if err := images.DownloadEpisodeThumb(f.Path, ep); err != nil {
						fmt.Fprintf(os.Stderr, "  warning: thumb: %v\n", err)
					}
				}
			}
			continue // skip group episode processing below
		}

		// Group episode processing
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
			if err := nfo.WriteEpisode(f.Path, ep, showID); err != nil {
				fmt.Fprintf(os.Stderr, "  warning: nfo: %v\n", err)
			}
			if !flagNoArt {
				if err := images.DownloadEpisodeThumb(f.Path, ep); err != nil {
					fmt.Fprintf(os.Stderr, "  warning: thumb: %v\n", err)
				}
			}
		}
	}

	fmt.Println("Done.")
	return nil
}

// ─── Movie ────────────────────────────────────────────────────────────────────

func processMovie(dir string, result *scanner.ScanResult, client *tmdb.Client, c *cache.Cache) error {
	var movieID int

	if entry, ok := c.Get(); ok && !flagForce {
		fmt.Printf("Cache hit: %s → TMDB %d\n", entry.Title, entry.TMDBID)
		movieID = entry.TMDBID
	} else {
		fmt.Printf("Searching TMDB for movie: %q (year: %d)\n", result.Title, result.Year)
		movies, err := client.SearchMovie(result.Title, result.Year)
		if err != nil {
			return err
		}
		if len(movies) == 0 {
			return fmt.Errorf("no results for %q", result.Title)
		}

		chosen, err := pickMovie(movies)
		if err != nil {
			return err
		}
		movieID = chosen.ID

		if !flagDryRun {
			resolvedTitle := chosen.Title
			if len(chosen.ReleaseDate) >= 4 {
				resolvedTitle += " (" + chosen.ReleaseDate[:4] + ")"
			}
			if err := c.Set("movie", resolvedTitle, chosen.ID, ""); err != nil {
				fmt.Fprintf(os.Stderr, "warning: could not save to cache: %v\n", err)
			}
		}
	}

	if flagDryRun {
		fmt.Println("[dry-run] Would write movie.nfo and download artwork")
		return nil
	}

	detail, err := client.GetMovie(movieID)
	if err != nil {
		return fmt.Errorf("fetching movie details: %w", err)
	}

	fmt.Printf("Matched: %s (%s)\n", detail.Title, detail.ReleaseDate[:min(4, len(detail.ReleaseDate))])

	fmt.Println("Writing movie.nfo …")
	if err := nfo.WriteMovie(dir, detail); err != nil {
		return err
	}
	if !flagNoArt {
		fmt.Println("Downloading artwork …")
		if err := images.DownloadMovie(dir, detail); err != nil {
			fmt.Fprintf(os.Stderr, "warning: %v\n", err)
		}
	}

	fmt.Println("Done.")
	return nil
}

// ─── Interactive pickers ──────────────────────────────────────────────────────

func pickTV(shows []tmdb.TVShow) (*tmdb.TVShow, error) {
	if len(shows) == 1 {
		fmt.Printf("Auto-selected: %s (%s) [TMDB %d]\n", shows[0].Name, shows[0].FirstAirDate, shows[0].ID)
		return &shows[0], nil
	}

	limit := shows
	if len(limit) > 8 {
		limit = limit[:8]
	}
	fmt.Println("\nMultiple results — pick one:")
	for i, s := range limit {
		year := ""
		if len(s.FirstAirDate) >= 4 {
			year = s.FirstAirDate[:4]
		}
		fmt.Printf("  [%d] %s (%s) — TMDB %d\n", i+1, s.Name, year, s.ID)
	}
	fmt.Printf("  [0] None / cancel\n")

	n, err := readInt("> ", 0, len(limit))
	if err != nil {
		return nil, err
	}
	if n == 0 {
		return nil, fmt.Errorf("cancelled")
	}
	return &limit[n-1], nil
}

func pickMovie(movies []tmdb.Movie) (*tmdb.Movie, error) {
	if len(movies) == 1 {
		fmt.Printf("Auto-selected: %s (%s) [TMDB %d]\n", movies[0].Title, movies[0].ReleaseDate, movies[0].ID)
		return &movies[0], nil
	}

	limit := movies
	if len(limit) > 8 {
		limit = limit[:8]
	}
	fmt.Println("\nMultiple results — pick one:")
	for i, m := range limit {
		year := ""
		if len(m.ReleaseDate) >= 4 {
			year = m.ReleaseDate[:4]
		}
		fmt.Printf("  [%d] %s (%s) — TMDB %d\n", i+1, m.Title, year, m.ID)
	}
	fmt.Printf("  [0] None / cancel\n")

	n, err := readInt("> ", 0, len(limit))
	if err != nil {
		return nil, err
	}
	if n == 0 {
		return nil, fmt.Errorf("cancelled")
	}
	return &limit[n-1], nil
}

func readInt(prompt string, min, max int) (int, error) {
	reader := bufio.NewReader(os.Stdin)
	for {
		fmt.Print(prompt)
		line, err := reader.ReadString('\n')
		if err != nil {
			return 0, err
		}
		line = strings.TrimSpace(line)
		n, err := strconv.Atoi(line)
		if err != nil || n < min || n > max {
			fmt.Printf("Enter a number between %d and %d\n", min, max)
			continue
		}
		return n, nil
	}
}

// ─── Helpers ──────────────────────────────────────────────────────────────────

func uniqueSeasons(files []*scanner.MediaFile) []int {
	seen := make(map[int]bool)
	for _, f := range files {
		seen[f.Season] = true
	}
	seasons := make([]int, 0, len(seen))
	for s := range seen {
		seasons = append(seasons, s)
	}
	sort.Ints(seasons)
	return seasons
}

func groupBySeasonDir(files []*scanner.MediaFile) map[string][]*scanner.MediaFile {
	m := make(map[string][]*scanner.MediaFile)
	for _, f := range files {
		m[f.Dir] = append(m[f.Dir], f)
	}
	return m
}

func pickEpisodeGroup(groups []tmdb.EpisodeGroupSummary) (string, error) {
	fmt.Println("\nEpisode groups available — pick one (or 0 for standard ordering):")
	for i, g := range groups {
		fmt.Printf("  [%d] %s (%s, %d episodes)\n", i+1, g.Name, g.TypeName(), g.EpisodeCount)
	}
	fmt.Println("  [0] None / use standard ordering")

	n, err := readInt("> ", 0, len(groups))
	if err != nil {
		return "", err
	}
	if n == 0 {
		return "", nil
	}
	return groups[n-1].ID, nil
}

var groupSeasonNameRe = regexp.MustCompile(`(?i)season\s*(\d+)`)

// groupSeasonNumber extracts the season number from a group season name
// ("Season 1" → 1, "Season 01" → 1). Falls back to Order+1 for unlabelled
// seasons, and returns 0 for specials/extras so they don't collide with S01.
func groupSeasonNumber(name string, order int) int {
	if m := groupSeasonNameRe.FindStringSubmatch(name); m != nil {
		n, _ := strconv.Atoi(m[1])
		return n
	}
	lower := strings.ToLower(name)
	if strings.Contains(lower, "special") || strings.Contains(lower, "extra") ||
		strings.Contains(lower, "ova") || strings.Contains(lower, "bonus") {
		return 0
	}
	return order + 1
}

// buildGroupMap returns a map of (groupSeasonNum, groupEpisodeNum) → Episode.
func buildGroupMap(group *tmdb.EpisodeGroup) map[[2]int]*tmdb.Episode {
	m := make(map[[2]int]*tmdb.Episode)
	for _, gs := range group.Groups {
		seasonNum := groupSeasonNumber(gs.Name, gs.Order)
		for _, gep := range gs.Episodes {
			epNum := gep.Order + 1
			ep := &tmdb.Episode{
				ID:            gep.ID,
				Name:          gep.Name,
				Overview:      gep.Overview,
				SeasonNumber:  seasonNum,
				EpisodeNumber: epNum,
				AirDate:       gep.AirDate,
				StillPath:     gep.StillPath,
				VoteAverage:   gep.VoteAverage,
				VoteCount:     gep.VoteCount,
				Runtime:       gep.Runtime,
			}
			m[[2]int{seasonNum, epNum}] = ep
		}
	}
	return m
}

func min(a, b int) int {
	if a < b {
		return a
	}
	return b
}
