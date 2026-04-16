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

func download(url, destPath string, force bool) error {
	if url == "" {
		return nil
	}

	if !force {
		if _, err := os.Stat(destPath); err == nil {
			return nil // already exists
		}
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
func DownloadTVShow(dir string, detail *tmdb.TVShowDetail, force bool) error {
	if err := download(tmdb.ImageURL(detail.PosterPath), filepath.Join(dir, "poster.jpg"), force); err != nil {
		return fmt.Errorf("show poster: %w", err)
	}
	if err := download(tmdb.ImageURL(detail.BackdropPath), filepath.Join(dir, "backdrop.jpg"), force); err != nil {
		return fmt.Errorf("show backdrop: %w", err)
	}
	return nil
}

// DownloadSeasonPosters downloads a poster for every season in the list.
// Filenames follow Jellyfin convention: season01-poster.jpg, season-specials-poster.jpg for season 0.
// Uses the SeasonSummary slice from TVShowDetail.Seasons so no extra API calls are needed.
func DownloadSeasonPosters(showDir string, seasons []tmdb.SeasonSummary, force bool) error {
	for _, s := range seasons {
		if s.PosterPath == "" {
			continue
		}
		var name string
		if s.SeasonNumber == 0 {
			name = "season-specials-poster.jpg"
		} else {
			name = fmt.Sprintf("season%02d-poster.jpg", s.SeasonNumber)
		}
		if err := download(tmdb.ImageURL(s.PosterPath), filepath.Join(showDir, name), force); err != nil {
			return fmt.Errorf("season %d poster: %w", s.SeasonNumber, err)
		}
	}
	return nil
}

// DownloadEpisodeThumb downloads the episode still image next to the video file.
// Filename: <video-base>-thumb.jpg
func DownloadEpisodeThumb(videoPath string, ep *tmdb.Episode, force bool) error {
	if ep.StillPath == "" {
		return nil
	}
	ext := filepath.Ext(videoPath)
	thumbPath := videoPath[:len(videoPath)-len(ext)] + "-thumb.jpg"
	return download(tmdb.ImageURL(ep.StillPath), thumbPath, force)
}

// DownloadListPoster downloads the list poster into dir as poster.jpg.
func DownloadListPoster(dir string, list *tmdb.List, force bool) error {
	if err := download(tmdb.ImageURL(list.PosterPath), filepath.Join(dir, "poster.jpg"), force); err != nil {
		return fmt.Errorf("list poster: %w", err)
	}
	return nil
}

// DownloadListItemThumb downloads a list item's poster as the episode thumb next to the video file.
func DownloadListItemThumb(videoPath string, item *tmdb.ListItem, force bool) error {
	if item.PosterPath == "" {
		return nil
	}
	ext := filepath.Ext(videoPath)
	thumbPath := videoPath[:len(videoPath)-len(ext)] + "-thumb.jpg"
	return download(tmdb.ImageURL(item.PosterPath), thumbPath, force)
}

// DownloadMovie downloads poster and fanart for a movie into dir.
func DownloadMovie(dir string, detail *tmdb.MovieDetail, force bool) error {
	if err := download(tmdb.ImageURL(detail.PosterPath), filepath.Join(dir, "poster.jpg"), force); err != nil {
		return fmt.Errorf("movie poster: %w", err)
	}
	if err := download(tmdb.ImageURL(detail.BackdropPath), filepath.Join(dir, "backdrop.jpg"), force); err != nil {
		return fmt.Errorf("movie backdrop: %w", err)
	}
	return nil
}
