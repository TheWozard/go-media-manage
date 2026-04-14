package cache

import (
	"encoding/json"
	"fmt"
	"os"
	"path/filepath"
	"sync"
	"time"
)

type Entry struct {
	TMDBID    int       `json:"tmdb_id"`
	Title     string    `json:"title"`
	MediaType string    `json:"media_type"`
	MatchedAt time.Time `json:"matched_at"`
}

type Cache struct {
	mu      sync.RWMutex
	path    string
	entries map[string]*Entry // key: "tv:<normalised title>" or "movie:<normalised title>"
}

func New(dir string) (*Cache, error) {
	if err := os.MkdirAll(dir, 0755); err != nil {
		return nil, fmt.Errorf("creating cache dir: %w", err)
	}

	c := &Cache{
		path:    filepath.Join(dir, "matches.json"),
		entries: make(map[string]*Entry),
	}

	if err := c.load(); err != nil && !os.IsNotExist(err) {
		return nil, err
	}

	return c, nil
}

func (c *Cache) load() error {
	data, err := os.ReadFile(c.path)
	if err != nil {
		return err
	}
	return json.Unmarshal(data, &c.entries)
}

func (c *Cache) save() error {
	data, err := json.MarshalIndent(c.entries, "", "  ")
	if err != nil {
		return err
	}
	return os.WriteFile(c.path, data, 0644)
}

func key(mediaType, title string) string {
	return fmt.Sprintf("%s:%s", mediaType, title)
}

func (c *Cache) Get(mediaType, title string) (*Entry, bool) {
	c.mu.RLock()
	defer c.mu.RUnlock()
	e, ok := c.entries[key(mediaType, title)]
	return e, ok
}

func (c *Cache) Set(mediaType, title string, tmdbID int, resolvedTitle string) error {
	c.mu.Lock()
	defer c.mu.Unlock()

	c.entries[key(mediaType, title)] = &Entry{
		TMDBID:    tmdbID,
		Title:     resolvedTitle,
		MediaType: mediaType,
		MatchedAt: time.Now(),
	}

	return c.save()
}

func (c *Cache) Delete(mediaType, title string) error {
	c.mu.Lock()
	defer c.mu.Unlock()
	delete(c.entries, key(mediaType, title))
	return c.save()
}
