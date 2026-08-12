package cmd

import (
	"fmt"

	"github.com/spf13/cobra"
	"go-media-manage/internal/version"
)

var versionCmd = &cobra.Command{
	Use:   "version",
	Short: "Print the gmm version",
	Run: func(cmd *cobra.Command, args []string) {
		fmt.Printf("gmm %s (%s, built %s)\n", version.Version, version.Commit, version.Date)
	},
}

func init() {
	rootCmd.AddCommand(versionCmd)
}
