package cmd

import (
	"fmt"

	"github.com/spf13/cobra"
	"go-media-manage/internal/config"
)

var configCmd = &cobra.Command{
	Use:   "config",
	Short: "Manage configuration",
}

var configShowCmd = &cobra.Command{
	Use:   "show",
	Short: "Show current configuration",
	RunE: func(cmd *cobra.Command, args []string) error {
		cfg, err := config.Load()
		if err != nil {
			return err
		}
		key := cfg.TMDBAPIKey
		if len(key) > 8 {
			key = key[:4] + "****" + key[len(key)-4:]
		} else if key != "" {
			key = "****"
		} else {
			key = "(not set)"
		}
		fmt.Printf("TMDB API key : %s\n", key)
		fmt.Printf("Language     : %s\n", cfg.Language)
		fmt.Printf("Cache dir    : %s\n", cfg.CacheDir)
		return nil
	},
}

var configSetKeyCmd = &cobra.Command{
	Use:   "set-key <api-key>",
	Short: "Set TMDB API key",
	Args:  cobra.ExactArgs(1),
	RunE: func(cmd *cobra.Command, args []string) error {
		cfg, err := config.Load()
		if err != nil {
			return err
		}
		cfg.TMDBAPIKey = args[0]
		if err := cfg.Save(); err != nil {
			return err
		}
		fmt.Println("API key saved.")
		return nil
	},
}

var configSetLangCmd = &cobra.Command{
	Use:   "set-language <language>",
	Short: "Set metadata language (e.g. en-US, de-DE)",
	Args:  cobra.ExactArgs(1),
	RunE: func(cmd *cobra.Command, args []string) error {
		cfg, err := config.Load()
		if err != nil {
			return err
		}
		cfg.Language = args[0]
		if err := cfg.Save(); err != nil {
			return err
		}
		fmt.Printf("Language set to %s.\n", args[0])
		return nil
	},
}

func init() {
	configCmd.AddCommand(configShowCmd, configSetKeyCmd, configSetLangCmd)
	rootCmd.AddCommand(configCmd)
}
