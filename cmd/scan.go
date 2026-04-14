package cmd

import (
	"bufio"
	"fmt"
	"os"
	"path/filepath"
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

	client := tmdb.NewClient(cfg.TMDBAPIKey, cfg.Language)
	c, err := cache.New(cfg.CacheDir)
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
	// Resolve TMDB ID
	var showID int

	if entry, ok := c.Get("tv", result.Title); ok && !flagForce {
		fmt.Printf("Cache hit: %s → TMDB %d\n", result.Title, entry.TMDBID)
		showID = entry.TMDBID
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

		if !flagDryRun {
			if err := c.Set("tv", result.Title, chosen.ID, chosen.Name); err != nil {
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
	fmt.Println("Downloading show artwork …")
	if err := images.DownloadTVShow(dir, detail); err != nil {
		fmt.Fprintf(os.Stderr, "warning: %v\n", err)
	}

	// Group files by season
	bySeasonDir := groupBySeasonDir(result.Files)

	// Determine which seasons we need
	seasons := uniqueSeasons(result.Files)

	for _, seasonNum := range seasons {
		fmt.Printf("Fetching season %d …\n", seasonNum)
		season, err := client.GetSeason(showID, seasonNum)
		if err != nil {
			fmt.Fprintf(os.Stderr, "warning: season %d: %v\n", seasonNum, err)
			continue
		}

		// Episode lookup map
		epMap := make(map[int]*tmdb.Episode)
		for i := range season.Episodes {
			epMap[season.Episodes[i].EpisodeNumber] = &season.Episodes[i]
		}

		// Write season NFO in the season's directory
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

		// Season poster goes into show root
		if err := images.DownloadSeason(dir, season); err != nil {
			fmt.Fprintf(os.Stderr, "  warning: season %d poster: %v\n", seasonNum, err)
		}

		// Process each episode file for this season
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
			if err := images.DownloadEpisodeThumb(f.Path, ep); err != nil {
				fmt.Fprintf(os.Stderr, "  warning: thumb: %v\n", err)
			}
		}
	}

	fmt.Println("Done.")
	return nil
}

// ─── Movie ────────────────────────────────────────────────────────────────────

func processMovie(dir string, result *scanner.ScanResult, client *tmdb.Client, c *cache.Cache) error {
	var movieID int

	cacheKey := result.Title
	if result.Year > 0 {
		cacheKey = fmt.Sprintf("%s (%d)", result.Title, result.Year)
	}

	if entry, ok := c.Get("movie", cacheKey); ok && !flagForce {
		fmt.Printf("Cache hit: %s → TMDB %d\n", cacheKey, entry.TMDBID)
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
			if err := c.Set("movie", cacheKey, chosen.ID, resolvedTitle); err != nil {
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
	fmt.Println("Downloading artwork …")
	if err := images.DownloadMovie(dir, detail); err != nil {
		fmt.Fprintf(os.Stderr, "warning: %v\n", err)
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

func min(a, b int) int {
	if a < b {
		return a
	}
	return b
}
