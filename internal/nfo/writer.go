package nfo

import (
	"encoding/xml"
	"fmt"
	"os"
	"path/filepath"
	"strings"

	"go-media-manage/internal/tmdb"
)

// --- TV Show NFO ---

type TVShowNFO struct {
	XMLName       xml.Name `xml:"tvshow"`
	Title         string   `xml:"title"`
	SortTitle     string   `xml:"sorttitle,omitempty"`
	Year          string   `xml:"year,omitempty"`
	Plot          string   `xml:"plot"`
	Tagline       string   `xml:"tagline,omitempty"`
	Rating        float64  `xml:"rating,omitempty"`
	Votes         int      `xml:"votes,omitempty"`
	TMDBID        int      `xml:"tmdbid"`
	IMDBID        string   `xml:"imdbid,omitempty"`
	TVDBID        int      `xml:"tvdbid,omitempty"`
	Status        string   `xml:"status,omitempty"`
	Studio        string   `xml:"studio,omitempty"`
	Genres        []string `xml:"genre"`
	MPAA          string   `xml:"mpaa,omitempty"`
}

// --- Season NFO ---

type SeasonNFO struct {
	XMLName      xml.Name `xml:"season"`
	Title        string   `xml:"title"`
	SeasonNumber int      `xml:"seasonnumber"`
	Year         string   `xml:"year,omitempty"`
	Plot         string   `xml:"plot"`
	TMDBID       int      `xml:"tmdbid,omitempty"`
}

// --- Episode NFO ---

type EpisodeNFO struct {
	XMLName       xml.Name  `xml:"episodedetails"`
	Title         string    `xml:"title"`
	Season        int       `xml:"season"`
	Episode       int       `xml:"episode"`
	Plot          string    `xml:"plot"`
	Rating        float64   `xml:"rating,omitempty"`
	Votes         int       `xml:"votes,omitempty"`
	Aired         string    `xml:"aired,omitempty"`
	Runtime       int       `xml:"runtime,omitempty"`
	TMDBID        int       `xml:"tmdbid,omitempty"`
	Actors        []Actor   `xml:"actor"`
}

// --- Movie NFO ---

type MovieNFO struct {
	XMLName   xml.Name `xml:"movie"`
	Title     string   `xml:"title"`
	SortTitle string   `xml:"sorttitle,omitempty"`
	Year      string   `xml:"year,omitempty"`
	Plot      string   `xml:"plot"`
	Tagline   string   `xml:"tagline,omitempty"`
	Rating    float64  `xml:"rating,omitempty"`
	Votes     int      `xml:"votes,omitempty"`
	Runtime   int      `xml:"runtime,omitempty"`
	TMDBID    int      `xml:"tmdbid"`
	IMDBID    string   `xml:"imdbid,omitempty"`
	Genres    []string `xml:"genre"`
	MPAA      string   `xml:"mpaa,omitempty"`
	Actors    []Actor  `xml:"actor"`
}

type Actor struct {
	Name  string `xml:"name"`
	Role  string `xml:"role,omitempty"`
	Order int    `xml:"order,omitempty"`
}

func write(path string, v interface{}) error {
	if err := os.MkdirAll(filepath.Dir(path), 0755); err != nil {
		return err
	}

	f, err := os.Create(path)
	if err != nil {
		return fmt.Errorf("creating %s: %w", path, err)
	}
	defer f.Close()

	f.WriteString(xml.Header)
	enc := xml.NewEncoder(f)
	enc.Indent("", "  ")
	if err := enc.Encode(v); err != nil {
		return fmt.Errorf("encoding %s: %w", path, err)
	}
	return enc.Close()
}

func airYear(dateStr string) string {
	if len(dateStr) >= 4 {
		return dateStr[:4]
	}
	return ""
}

func contentRating(detail *tmdb.TVShowDetail) string {
	for _, r := range detail.ContentRatings.Results {
		if r.ISO31661 == "US" {
			return "TV-" + r.Rating
		}
	}
	return ""
}

func movieRating(detail *tmdb.MovieDetail) string {
	for _, r := range detail.Releases.Countries {
		if r.ISO31661 == "US" && r.Certification != "" {
			return r.Certification
		}
	}
	return ""
}

func WriteTVShow(dir string, detail *tmdb.TVShowDetail) error {
	genres := make([]string, len(detail.Genres))
	for i, g := range detail.Genres {
		genres[i] = g.Name
	}

	studio := ""
	if len(detail.Networks) > 0 {
		studio = detail.Networks[0].Name
	}

	nfo := &TVShowNFO{
		Title:   detail.Name,
		Year:    airYear(detail.FirstAirDate),
		Plot:    detail.Overview,
		Tagline: detail.Tagline,
		Rating:  detail.VoteAverage,
		Votes:   detail.VoteCount,
		TMDBID:  detail.ID,
		IMDBID:  detail.ExternalIDs.IMDBID,
		TVDBID:  detail.ExternalIDs.TVDBID,
		Status:  detail.Status,
		Studio:  studio,
		Genres:  genres,
		MPAA:    contentRating(detail),
	}

	return write(filepath.Join(dir, "tvshow.nfo"), nfo)
}

func WriteSeason(dir string, season *tmdb.Season, showID int) error {
	nfo := &SeasonNFO{
		Title:        season.Name,
		SeasonNumber: season.SeasonNumber,
		Year:         airYear(season.AirDate),
		Plot:         season.Overview,
		TMDBID:       showID,
	}
	return write(filepath.Join(dir, "season.nfo"), nfo)
}

func WriteEpisode(videoPath string, ep *tmdb.Episode, showTMDBID int) error {
	nfoPath := strings.TrimSuffix(videoPath, filepath.Ext(videoPath)) + ".nfo"

	actors := make([]Actor, 0, len(ep.GuestStars))
	for i, c := range ep.GuestStars {
		if i >= 10 {
			break
		}
		actors = append(actors, Actor{Name: c.Name, Role: c.Character, Order: c.Order})
	}

	nfo := &EpisodeNFO{
		Title:   ep.Name,
		Season:  ep.SeasonNumber,
		Episode: ep.EpisodeNumber,
		Plot:    ep.Overview,
		Rating:  ep.VoteAverage,
		Votes:   ep.VoteCount,
		Aired:   ep.AirDate,
		Runtime: ep.Runtime,
		TMDBID:  showTMDBID,
		Actors:  actors,
	}

	return write(nfoPath, nfo)
}

func WriteMovie(dir string, detail *tmdb.MovieDetail) error {
	genres := make([]string, len(detail.Genres))
	for i, g := range detail.Genres {
		genres[i] = g.Name
	}

	actors := make([]Actor, 0, len(detail.Credits.Cast))
	for i, c := range detail.Credits.Cast {
		if i >= 10 {
			break
		}
		actors = append(actors, Actor{Name: c.Name, Role: c.Character, Order: c.Order})
	}

	nfo := &MovieNFO{
		Title:   detail.Title,
		Year:    airYear(detail.ReleaseDate),
		Plot:    detail.Overview,
		Tagline: detail.Tagline,
		Rating:  detail.VoteAverage,
		Votes:   detail.VoteCount,
		Runtime: detail.Runtime,
		TMDBID:  detail.ID,
		IMDBID:  detail.ExternalIDs.IMDBID,
		Genres:  genres,
		MPAA:    movieRating(detail),
		Actors:  actors,
	}

	return write(filepath.Join(dir, "movie.nfo"), nfo)
}
