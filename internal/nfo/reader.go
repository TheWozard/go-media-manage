package nfo

import (
	"encoding/xml"
	"fmt"
	"os"
)

type TVShowInfo struct {
	Title string `xml:"title"`
}

type EpisodeInfo struct {
	Title   string `xml:"title"`
	Season  int    `xml:"season"`
	Episode int    `xml:"episode"`
}

type MovieInfo struct {
	Title string `xml:"title"`
	Year  string `xml:"year"`
}

func readXML(path string, out interface{}) error {
	data, err := os.ReadFile(path)
	if err != nil {
		return fmt.Errorf("reading %s: %w", path, err)
	}
	return xml.Unmarshal(data, out)
}

func ReadTVShow(path string) (*TVShowInfo, error) {
	var v struct {
		XMLName xml.Name `xml:"tvshow"`
		TVShowInfo
	}
	if err := readXML(path, &v); err != nil {
		return nil, err
	}
	return &v.TVShowInfo, nil
}

func ReadEpisode(path string) (*EpisodeInfo, error) {
	var v struct {
		XMLName xml.Name `xml:"episodedetails"`
		EpisodeInfo
	}
	if err := readXML(path, &v); err != nil {
		return nil, err
	}
	return &v.EpisodeInfo, nil
}

func ReadMovie(path string) (*MovieInfo, error) {
	var v struct {
		XMLName xml.Name `xml:"movie"`
		MovieInfo
	}
	if err := readXML(path, &v); err != nil {
		return nil, err
	}
	return &v.MovieInfo, nil
}
