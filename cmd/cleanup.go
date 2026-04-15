package cmd

import (
	"fmt"
	"os"
	"path/filepath"
	"strings"

	"github.com/spf13/cobra"
)

var cleanupCmd = &cobra.Command{
	Use:   "cleanup <directory>",
	Short: "Move non-MKV files to an archive folder",
	Long: `Moves every non-MKV file (NFO, JPG, JSON, etc.) into an 'archive'
subfolder at the root of the directory, preserving relative paths.`,
	Args: cobra.ExactArgs(1),
	RunE: runCleanup,
}

func init() {
	rootCmd.AddCommand(cleanupCmd)
}

func runCleanup(cmd *cobra.Command, args []string) error {
	dir, err := filepath.Abs(args[0])
	if err != nil {
		return err
	}

	archiveDir := filepath.Join(dir, "archive")
	var moved, skipped int

	err = filepath.WalkDir(dir, func(path string, d os.DirEntry, err error) error {
		if err != nil {
			return nil
		}
		if d.IsDir() {
			// Don't descend into the archive folder itself
			if path == archiveDir {
				return filepath.SkipDir
			}
			return nil
		}

		if strings.ToLower(filepath.Ext(path)) == ".mkv" {
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

	fmt.Printf("\n%d file(s) moved, %d skipped.\n", moved, skipped)
	return nil
}
