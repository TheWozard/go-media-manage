package cmd

import (
	"fmt"
	"os"
	"path/filepath"
	"strings"

	"github.com/spf13/cobra"
	"go-media-manage/internal/nfo"
	"go-media-manage/internal/scanner"
	"go-media-manage/internal/scope"
)

var flagRenameRoot bool

var renameCmd = &cobra.Command{
	Use:   "rename <directory>",
	Short: "Rename media files using NFO metadata",
	Long: `Reads NFO files in the directory and renames the matching video file,
NFO, and thumbnail to a clean standard format.

TV:    Show Name S01E01 - Episode Title.mkv
Movie: Movie Title (2010).mkv

Scope is inferred from the directory name: a "Season N" directory targets
that season only; any other directory targets everything. Use --root to
restrict to show-level files only.`,
	Args: cobra.ExactArgs(1),
	RunE: runRename,
}

func init() {
	renameCmd.Flags().BoolVar(&flagRenameRoot, "root", false, "Restrict to show-level files only")
	rootCmd.AddCommand(renameCmd)
}

func runRename(cmd *cobra.Command, args []string) error {
	dir, err := filepath.Abs(args[0])
	if err != nil {
		return err
	}

	var sc scope.Scope
	if flagRenameRoot {
		sc = scope.Root()
	} else {
		sc = scope.FromDir(dir)
	}

	rootDir := sc.RootDir(dir)

	// Detect mode by looking for tvshow.nfo or movie.nfo in the show root
	if _, err := os.Stat(filepath.Join(rootDir, "tvshow.nfo")); err == nil {
		return renameTVShow(dir, sc)
	}
	if _, err := os.Stat(filepath.Join(rootDir, "movie.nfo")); err == nil {
		return renameMovie(dir, sc)
	}

	return fmt.Errorf("no tvshow.nfo or movie.nfo found in %s — run 'pull' first", rootDir)
}

// ─── TV ──────────────────────────────────────────────────────────────────────

func renameTVShow(dir string, sc scope.Scope) error {
	show, err := nfo.ReadTVShow(filepath.Join(sc.RootDir(dir), "tvshow.nfo"))
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

	err = sc.WalkDir(dir, func(path string, d os.DirEntry) error {
		if d.IsDir() {
			if n := scanner.ParseSeasonDir(path); n > 0 {
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

func renameMovie(dir string, sc scope.Scope) error {
	if sc.Season() > 0 {
		return fmt.Errorf("season scope is not valid for movies")
	}
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
	for ext := range scanner.VideoExts {
		candidates = append(candidates, struct{ old, new string }{oldBase + ext, newBase + ext})
	}

	for _, pair := range candidates {
		oldPath := filepath.Join(dir, pair.old)
		newPath := filepath.Join(dir, pair.new)

		if _, err := os.Stat(oldPath); os.IsNotExist(err) {
			continue
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
