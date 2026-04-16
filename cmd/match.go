package cmd

import (
	"bufio"
	"fmt"
	"os"
	"path/filepath"
	"strconv"
	"strings"

	"github.com/spf13/cobra"
	"go-media-manage/internal/cache"
	"go-media-manage/internal/config"
	"go-media-manage/internal/scanner"
	"go-media-manage/internal/tmdb"
)

var (
	flagMatchType   string
	flagMatchListID int
)

var matchCmd = &cobra.Command{
	Use:   "match <directory>",
	Short: "Match a directory against TMDB and cache the result",
	Long: `Scans a directory, searches TMDB for the best match, and saves the
result to matches.json inside the directory. Run 'pull' afterwards to
download metadata and artwork.

Use --list-id to treat a TMDB list as a single-season TV series. Each item
in the list becomes an episode, ordered by release date.`,
	Args: cobra.ExactArgs(1),
	RunE: runMatch,
}

func init() {
	matchCmd.Flags().StringVarP(&flagMatchType, "type", "t", "auto", "Media type: auto, tv, movie")
	matchCmd.Flags().IntVar(&flagMatchListID, "list-id", 0, "TMDB list ID to treat as a single-season TV series")
	rootCmd.AddCommand(matchCmd)
}

func runMatch(cmd *cobra.Command, args []string) error {
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

	hint := scanner.MediaType(flagMatchType)
	if hint != scanner.TypeTV && hint != scanner.TypeMovie && hint != scanner.TypeAuto {
		return fmt.Errorf("--type must be auto, tv, or movie")
	}

	result, err := scanner.Scan(dir, hint)
	if err != nil {
		return err
	}

	client := tmdb.NewClient(cfg.TMDBToken, cfg.Language)
	c, err := cache.New(dir)
	if err != nil {
		return err
	}

	if flagMatchListID != 0 {
		return matchList(flagMatchListID, result, client, c)
	}

	fmt.Printf("Found %d video file(s) — detected type: %s\n", len(result.Files), result.MediaType)

	switch result.MediaType {
	case scanner.TypeTV:
		return matchTV(result, client, c)
	case scanner.TypeMovie:
		return matchMovie(result, client, c)
	default:
		return fmt.Errorf("unknown media type")
	}
}

func matchList(listID int, result *scanner.ScanResult, client *tmdb.Client, c *cache.Cache) error {
	list, err := client.GetList(listID)
	if err != nil {
		return fmt.Errorf("fetching list %d: %w", listID, err)
	}

	fmt.Printf("List: %q — %d item(s)\n", list.Name, len(list.Items))
	if list.Description != "" {
		fmt.Printf("      %s\n", list.Description)
	}

	// Lists have no poster/backdrop, so anchor the show root to a movie entry.
	fmt.Printf("\nSearching TMDB for movie: %q (year: %d)\n", result.Title, result.Year)
	movies, err := client.SearchMovie(result.Title, result.Year)
	if err != nil {
		return fmt.Errorf("searching movies: %w", err)
	}
	if len(movies) == 0 {
		return fmt.Errorf("no movie results for %q", result.Title)
	}

	movie, err := pickMovie(movies)
	if err != nil {
		return err
	}

	if err := c.SetList(list.Name, movie.ID, list.ID); err != nil {
		fmt.Fprintf(os.Stderr, "warning: could not save match: %v\n", err)
	}
	fmt.Printf("\nMatched: %s — root movie: %s [TMDB %d], season 1 from list %d\n",
		list.Name, movie.Title, movie.ID, list.ID)
	return nil
}

func matchTV(result *scanner.ScanResult, client *tmdb.Client, c *cache.Cache) error {
	fmt.Printf("Searching TMDB for TV show: %q\n", result.Title)
	shows, err := client.SearchTV(result.Title, result.Year)
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

	groups, err := client.GetEpisodeGroups(chosen.ID)
	if err == nil && len(groups) > 0 {
		groupID, err := pickEpisodeGroup(groups)
		if err != nil {
			return err
		}
		if err := c.Set("tv", chosen.Name, chosen.ID, groupID); err != nil {
			fmt.Fprintf(os.Stderr, "warning: could not save match: %v\n", err)
		}
		if groupID != "" {
			fmt.Printf("\nMatched: %s [TMDB %d], episode group: %s\n", chosen.Name, chosen.ID, groupID)
		} else {
			fmt.Printf("\nMatched: %s [TMDB %d]\n", chosen.Name, chosen.ID)
		}
	} else {
		if err := c.Set("tv", chosen.Name, chosen.ID, ""); err != nil {
			fmt.Fprintf(os.Stderr, "warning: could not save match: %v\n", err)
		}
		fmt.Printf("\nMatched: %s [TMDB %d]\n", chosen.Name, chosen.ID)
	}

	return nil
}

func matchMovie(result *scanner.ScanResult, client *tmdb.Client, c *cache.Cache) error {
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

	resolvedTitle := chosen.Title
	if len(chosen.ReleaseDate) >= 4 {
		resolvedTitle += " (" + chosen.ReleaseDate[:4] + ")"
	}
	if err := c.Set("movie", resolvedTitle, chosen.ID, ""); err != nil {
		fmt.Fprintf(os.Stderr, "warning: could not save match: %v\n", err)
	}

	fmt.Printf("\nMatched: %s [TMDB %d]\n", resolvedTitle, chosen.ID)
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
