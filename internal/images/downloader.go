package images

import (
	"fmt"
	"io"
	"net/http"
	"os"
	"path/filepath"
	"time"

	"go-media-manage/internal/tmdb"
)

var httpClient = &http.Client{Timeout: 60 * time.Second}

func download(url, destPath string) error {
	if url == "" {
		return nil
	}

	// Skip if already exists
	if _, err := os.Stat(destPath); err == nil {
		return nil
	}

	resp, err := httpClient.Get(url)
	if err != nil {
		return fmt.Errorf("downloading %s: %w", url, err)
	}
	defer resp.Body.Close()

	if resp.StatusCode != 200 {
		return fmt.Errorf("image server returned %d for %s", resp.StatusCode, url)
	}

	if err := os.MkdirAll(filepath.Dir(destPath), 0755); err != nil {
		return err
	}

	f, err := os.Create(destPath)
	if err != nil {
		return err
	}
	defer f.Close()

	_, err = io.Copy(f, resp.Body)
	return err
}

// DownloadTVShow downloads poster and fanart for a show into dir.
func DownloadTVShow(dir string, detail *tmdb.TVShowDetail) error {
	if err := download(tmdb.ImageURL(detail.PosterPath), filepath.Join(dir, "poster.jpg")); err != nil {
		return fmt.Errorf("show poster: %w", err)
	}
	if err := download(tmdb.ImageURL(detail.BackdropPath), filepath.Join(dir, "fanart.jpg")); err != nil {
		return fmt.Errorf("show fanart: %w", err)
	}
	return nil
}

// DownloadSeason downloads the season poster into dir.
// Filename follows Kodi convention: season01-poster.jpg, season00-poster.jpg for specials.
func DownloadSeason(showDir string, season *tmdb.Season) error {
	if season.PosterPath == "" {
		return nil
	}
	var name string
	if season.SeasonNumber == 0 {
		name = "season00-poster.jpg"
	} else {
		name = fmt.Sprintf("season%02d-poster.jpg", season.SeasonNumber)
	}
	return download(tmdb.ImageURL(season.PosterPath), filepath.Join(showDir, name))
}

// DownloadEpisodeThumb downloads the episode still image next to the video file.
// Filename: <video-base>-thumb.jpg
func DownloadEpisodeThumb(videoPath string, ep *tmdb.Episode) error {
	if ep.StillPath == "" {
		return nil
	}
	ext := filepath.Ext(videoPath)
	thumbPath := videoPath[:len(videoPath)-len(ext)] + "-thumb.jpg"
	return download(tmdb.ImageURL(ep.StillPath), thumbPath)
}

// DownloadMovie downloads poster and fanart for a movie into dir.
func DownloadMovie(dir string, detail *tmdb.MovieDetail) error {
	if err := download(tmdb.ImageURL(detail.PosterPath), filepath.Join(dir, "poster.jpg")); err != nil {
		return fmt.Errorf("movie poster: %w", err)
	}
	if err := download(tmdb.ImageURL(detail.BackdropPath), filepath.Join(dir, "fanart.jpg")); err != nil {
		return fmt.Errorf("movie fanart: %w", err)
	}
	return nil
}
