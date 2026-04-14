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
		tok := cfg.TMDBToken
		if len(tok) > 8 {
			tok = tok[:4] + "****" + tok[len(tok)-4:]
		} else if tok != "" {
			tok = "****"
		} else {
			tok = "(not set)"
		}
		fmt.Printf("TMDB token : %s\n", tok)
		fmt.Printf("Language   : %s\n", cfg.Language)
		return nil
	},
}

var configSetTokenCmd = &cobra.Command{
	Use:   "set-token <read-access-token>",
	Short: "Set TMDB Read Access Token",
	Args:  cobra.ExactArgs(1),
	RunE: func(cmd *cobra.Command, args []string) error {
		cfg, err := config.Load()
		if err != nil {
			return err
		}
		cfg.TMDBToken = args[0]
		if err := cfg.Save(); err != nil {
			return err
		}
		fmt.Println("Read Access Token saved.")
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
	configCmd.AddCommand(configShowCmd, configSetTokenCmd, configSetLangCmd)
	rootCmd.AddCommand(configCmd)
}
