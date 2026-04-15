package cmd

import (
	"fmt"
	"os"
	"path/filepath"
	"strings"

	"github.com/spf13/cobra"
	"go-media-manage/internal/scope"
)

var flagCleanupRoot bool

var cleanupCmd = &cobra.Command{
	Use:   "cleanup <directory>",
	Short: "Move non-MKV files to an archive folder",
	Long: `Moves every non-MKV file (NFO, JPG, JSON, etc.) into an 'archive'
subfolder at the root of the directory, preserving relative paths.

Scope is inferred from the directory name: a "Season N" directory targets
that season only; any other directory targets everything. Use --root to
restrict to root-level files only.`,
	Args: cobra.ExactArgs(1),
	RunE: runCleanup,
}

func init() {
	cleanupCmd.Flags().BoolVar(&flagCleanupRoot, "root", false, "Restrict to root-level files only")
	rootCmd.AddCommand(cleanupCmd)
}

func runCleanup(cmd *cobra.Command, args []string) error {
	dir, err := filepath.Abs(args[0])
	if err != nil {
		return err
	}

	var sc scope.Scope
	if flagCleanupRoot {
		sc = scope.Root()
	} else {
		sc = scope.FromDir(dir)
	}

	archiveDir := filepath.Join(dir, ".archive")
	var moved, skipped int

	err = sc.WalkDir(dir, func(path string, d os.DirEntry) error {
		if d.IsDir() {
			return nil
		}
		if strings.ToLower(filepath.Ext(path)) == ".mkv" {
			return nil
		}
		if filepath.Base(path) == "matches.json" {
			return nil
		}

		rel, err := filepath.Rel(dir, path)
		if err != nil {
			return err
		}

		dest := filepath.Join(archiveDir, rel)

		fmt.Printf("  %s\n    → archive/%s\n", rel, rel)

		if err := os.MkdirAll(filepath.Dir(dest), 0755); err != nil {
			fmt.Fprintf(os.Stderr, "  error: %v\n", err)
			skipped++
			return nil
		}
		if err := os.Rename(path, dest); err != nil {
			fmt.Fprintf(os.Stderr, "  error: %v\n", err)
			skipped++
			return nil
		}
		moved++
		return nil
	})
	if err != nil {
		return err
	}

	// Collect all subdirectories (top-down), then remove empty ones bottom-up.
	var dirs []string
	filepath.WalkDir(dir, func(path string, d os.DirEntry, err error) error {
		if err != nil {
			return nil
		}
		if path == archiveDir {
			return filepath.SkipDir
		}
		if !d.IsDir() || path == dir {
			return nil
		}
		dirs = append(dirs, path)
		return nil
	})

	var removed int
	for i := len(dirs) - 1; i >= 0; i-- {
		entries, err := os.ReadDir(dirs[i])
		if err != nil || len(entries) > 0 {
			continue
		}
		rel, _ := filepath.Rel(dir, dirs[i])
		fmt.Printf("  removing empty dir: %s/\n", rel)
		if err := os.Remove(dirs[i]); err != nil {
			fmt.Fprintf(os.Stderr, "  error removing dir: %v\n", err)
		}
		removed++
	}

	fmt.Printf("\n%d file(s) moved, %d skipped, %d empty dir(s) removed.\n", moved, skipped, removed)
	return nil
}
