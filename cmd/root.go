package cmd

import (
	"fmt"
	"os"

	"github.com/spf13/cobra"
)

var rootCmd = &cobra.Command{
	Use:   "go-media-manage",
	Short: "CLI media manager — fetch metadata and artwork from TMDB",
	Long: `go-media-manage scans a directory for video files, matches them against
TMDB, and writes Jellyfin-compatible NFO files and artwork (poster, backdrop, thumbs).`,
}

func Execute() {
	if err := rootCmd.Execute(); err != nil {
		fmt.Fprintln(os.Stderr, err)
		os.Exit(1)
	}
}
