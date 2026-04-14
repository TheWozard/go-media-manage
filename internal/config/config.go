package config

import (
	"encoding/json"
	"fmt"
	"os"
	"path/filepath"
)

type Config struct {
	TMDBAPIKey string `json:"tmdb_api_key"`
	Language   string `json:"language"`
	CacheDir   string `json:"cache_dir"`
}

func configPath() (string, error) {
	home, err := os.UserHomeDir()
	if err != nil {
		return "", err
	}
	return filepath.Join(home, ".config", "go-media-manage", "config.json"), nil
}

func DefaultCacheDir() (string, error) {
	home, err := os.UserHomeDir()
	if err != nil {
		return "", err
	}
	return filepath.Join(home, ".cache", "go-media-manage"), nil
}

func Load() (*Config, error) {
	path, err := configPath()
	if err != nil {
		return nil, err
	}

	cacheDir, err := DefaultCacheDir()
	if err != nil {
		return nil, err
	}

	cfg := &Config{
		Language: "en-US",
		CacheDir: cacheDir,
	}

	data, err := os.ReadFile(path)
	if os.IsNotExist(err) {
		return cfg, nil
	}
	if err != nil {
		return nil, fmt.Errorf("reading config: %w", err)
	}

	if err := json.Unmarshal(data, cfg); err != nil {
		return nil, fmt.Errorf("parsing config: %w", err)
	}

	// Fill defaults for fields not in file
	if cfg.Language == "" {
		cfg.Language = "en-US"
	}
	if cfg.CacheDir == "" {
		cfg.CacheDir = cacheDir
	}

	return cfg, nil
}

func (c *Config) Save() error {
	path, err := configPath()
	if err != nil {
		return err
	}

	if err := os.MkdirAll(filepath.Dir(path), 0755); err != nil {
		return fmt.Errorf("creating config dir: %w", err)
	}

	data, err := json.MarshalIndent(c, "", "  ")
	if err != nil {
		return err
	}

	return os.WriteFile(path, data, 0600)
}

func (c *Config) Validate() error {
	if c.TMDBAPIKey == "" {
		return fmt.Errorf("TMDB API key not set — run: go-media-manage config set-key <key>")
	}
	return nil
}
