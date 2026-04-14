package cache

import (
	"encoding/json"
	"fmt"
	"os"
	"path/filepath"
	"time"
)

type Entry struct {
	TMDBID         int       `json:"tmdb_id"`
	Title          string    `json:"title"`
	MediaType      string    `json:"media_type"`
	EpisodeGroupID string    `json:"episode_group_id,omitempty"`
	MatchedAt      time.Time `json:"matched_at"`
}

type Cache struct {
	path string
}

func New(dir string) (*Cache, error) {
	if err := os.MkdirAll(dir, 0755); err != nil {
		return nil, fmt.Errorf("creating cache dir: %w", err)
	}
	return &Cache{path: filepath.Join(dir, "matches.json")}, nil
}

func (c *Cache) Get() (*Entry, bool) {
	data, err := os.ReadFile(c.path)
	if err != nil {
		return nil, false
	}
	var e Entry
	if err := json.Unmarshal(data, &e); err != nil {
		return nil, false
	}
	return &e, true
}

func (c *Cache) Set(mediaType, title string, tmdbID int, episodeGroupID string) error {
	e := &Entry{
		TMDBID:         tmdbID,
		Title:          title,
		MediaType:      mediaType,
		EpisodeGroupID: episodeGroupID,
		MatchedAt:      time.Now(),
	}
	data, err := json.MarshalIndent(e, "", "  ")
	if err != nil {
		return err
	}
	return os.WriteFile(c.path, data, 0644)
}
