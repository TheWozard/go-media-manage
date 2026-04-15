package cmd

import (
	"fmt"
	"os"
	"path/filepath"
	"regexp"
	"strconv"
	"strings"

	"github.com/spf13/cobra"
	"go-media-manage/internal/nfo"
	"go-media-manage/internal/scanner"
)

var renameCmd = &cobra.Command{
	Use:   "rename <directory>",
	Short: "Rename media files using NFO metadata",
	Long: `Reads NFO files in the directory and renames the matching video file,
NFO, and thumbnail to a clean standard format.

TV:    Show Name S01E01 - Episode Title.mkv
Movie: Movie Title (2010).mkv`,
	Args: cobra.ExactArgs(1),
	RunE: runRename,
}

func init() {
	rootCmd.AddCommand(renameCmd)
}

func runRename(cmd *cobra.Command, args []string) error {
	dir, err := filepath.Abs(args[0])
	if err != nil {
		return err
	}

	// Detect mode by looking for tvshow.nfo or movie.nfo in the root
	if _, err := os.Stat(filepath.Join(dir, "tvshow.nfo")); err == nil {
		return renameTVShow(dir)
	}
	if _, err := os.Stat(filepath.Join(dir, "movie.nfo")); err == nil {
		return renameMovie(dir)
	}

	return fmt.Errorf("no tvshow.nfo or movie.nfo found in %s — run 'pull' first", dir)
}

// ─── TV ──────────────────────────────────────────────────────────────────────

func renameTVShow(dir string) error {
	show, err := nfo.ReadTVShow(filepath.Join(dir, "tvshow.nfo"))
	if err != nil {
		return fmt.Errorf("reading tvshow.nfo: %w", err)
	}
	if show.Title == "" {
		return fmt.Errorf("tvshow.nfo has no title")
	}

	var renamed, skipped int
	type seasonDirEntry struct {
		path string
		n    int
	}
	var seasonDirs []seasonDirEntry

	err = filepath.WalkDir(dir, func(path string, d os.DirEntry, err error) error {
		if err != nil {
			return err
		}
		if d.IsDir() {
			if path == dir {
				return nil
			}
			if n := parseSeasonName(filepath.Base(path)); n > 0 {
				seasonDirs = append(seasonDirs, seasonDirEntry{path, n})
			}
			return nil
		}
		if filepath.Ext(path) != ".nfo" {
			return nil
		}

		base := strings.TrimSuffix(filepath.Base(path), ".nfo")
		if base == "tvshow" || base == "season" {
			return nil
		}

		ep, err := nfo.ReadEpisode(path)
		if err != nil || ep.Title == "" || ep.Season == 0 && ep.Episode == 0 {
			return nil
		}

		newBase := fmt.Sprintf("%s - S%02dE%02d - %s",
			safeName(show.Title), ep.Season, ep.Episode, safeName(ep.Title))

		n, s := renameGroup(filepath.Dir(path), base, newBase)
		renamed += n
		skipped += s
		return nil
	})
	if err != nil {
		return err
	}

	// Rename season directories after files are done
	for _, sd := range seasonDirs {
		standard := fmt.Sprintf("Season %02d", sd.n)
		if filepath.Base(sd.path) == standard {
			skipped++
			continue
		}
		newPath := filepath.Join(filepath.Dir(sd.path), standard)
		fmt.Printf("  %s/\n    → %s/\n", filepath.Base(sd.path), standard)
		if err := os.Rename(sd.path, newPath); err != nil {
			fmt.Fprintf(os.Stderr, "  error renaming dir: %v\n", err)
			skipped++
			continue
		}
		renamed++
	}

	fmt.Printf("\n%d file(s)/dir(s) renamed, %d skipped.\n", renamed, skipped)
	return nil
}

// ─── Movie ────────────────────────────────────────────────────────────────────

func renameMovie(dir string) error {
	movie, err := nfo.ReadMovie(filepath.Join(dir, "movie.nfo"))
	if err != nil {
		return fmt.Errorf("reading movie.nfo: %w", err)
	}
	if movie.Title == "" {
		return fmt.Errorf("movie.nfo has no title")
	}

	newBase := safeName(movie.Title)
	if movie.Year != "" {
		newBase += " (" + movie.Year + ")"
	}

	// Find the video file — take the first one that isn't already named correctly
	entries, err := os.ReadDir(dir)
	if err != nil {
		return err
	}

	var renamed, skipped int
	for _, e := range entries {
		if e.IsDir() {
			continue
		}
		ext := strings.ToLower(filepath.Ext(e.Name()))
		if !scanner.VideoExts[ext] {
			continue
		}
		oldBase := strings.TrimSuffix(e.Name(), filepath.Ext(e.Name()))
		n, s := renameGroup(dir, oldBase, newBase)
		renamed += n
		skipped += s
	}

	fmt.Printf("\n%d file(s) renamed, %d skipped.\n", renamed, skipped)
	return nil
}

// ─── Helpers ─────────────────────────────────────────────────────────────────

// renameGroup renames all related files sharing oldBase in dir to newBase.
// Related files: the video, .nfo, and -thumb.jpg.
// Returns (renamed, skipped) counts.
func renameGroup(dir, oldBase, newBase string) (renamed, skipped int) {
	candidates := []struct{ old, new string }{
		{oldBase + ".nfo", newBase + ".nfo"},
		{oldBase + "-thumb.jpg", newBase + "-thumb.jpg"},
	}
	// Add every video extension variant
	for ext := range scanner.VideoExts {
		candidates = append(candidates, struct{ old, new string }{oldBase + ext, newBase + ext})
	}

	for _, pair := range candidates {
		oldPath := filepath.Join(dir, pair.old)
		newPath := filepath.Join(dir, pair.new)

		if _, err := os.Stat(oldPath); os.IsNotExist(err) {
			continue // file doesn't exist, nothing to do
		}
		if pair.old == pair.new {
			skipped++
			continue
		}
		if _, err := os.Stat(newPath); err == nil {
			fmt.Printf("  skip (exists): %s\n", pair.new)
			skipped++
			continue
		}

		fmt.Printf("  %s\n    → %s\n", pair.old, pair.new)
		if err := os.Rename(oldPath, newPath); err != nil {
			fmt.Fprintf(os.Stderr, "  error: %v\n", err)
			skipped++
			continue
		}
		renamed++
	}
	return
}

// parseSeasonName extracts the season number from a directory name such as
// "Season 1", "Season 01", "S01", or "s2". Returns 0 if not recognised.
var seasonDirRe = regexp.MustCompile(`(?i)^s(?:eason\s*)?(\d{1,2})$`)

func parseSeasonName(name string) int {
	m := seasonDirRe.FindStringSubmatch(strings.TrimSpace(name))
	if m == nil {
		return 0
	}
	n, _ := strconv.Atoi(m[1])
	return n
}

// safeName strips characters that are illegal on common filesystems.
func safeName(s string) string {
	var b strings.Builder
	for _, r := range s {
		switch r {
		case '/', '\\', ':', '*', '?', '"', '<', '>', '|':
			b.WriteRune('-')
		default:
			b.WriteRune(r)
		}
	}
	return strings.TrimSpace(b.String())
}
