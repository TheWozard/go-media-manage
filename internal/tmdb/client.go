package tmdb

import (
	"encoding/json"
	"fmt"
	"io"
	"net/http"
	"net/url"
	"time"
)

const (
	baseURL   = "https://api.themoviedb.org/3"
	imageBase = "https://image.tmdb.org/t/p/original"
)

type Client struct {
	token      string
	language   string
	httpClient *http.Client
}

func NewClient(token, language string) *Client {
	return &Client{
		token:    token,
		language: language,
		httpClient: &http.Client{
			Timeout: 30 * time.Second,
		},
	}
}

func (c *Client) get(path string, params url.Values, out interface{}) error {
	if params == nil {
		params = url.Values{}
	}
	params.Set("language", c.language)

	u := fmt.Sprintf("%s%s?%s", baseURL, path, params.Encode())
	req, err := http.NewRequest(http.MethodGet, u, nil)
	if err != nil {
		return fmt.Errorf("building request: %w", err)
	}
	req.Header.Set("Authorization", "Bearer "+c.token)
	req.Header.Set("Accept", "application/json")

	resp, err := c.httpClient.Do(req)
	if err != nil {
		return fmt.Errorf("HTTP request failed: %w", err)
	}
	defer resp.Body.Close()

	if resp.StatusCode == 401 {
		return fmt.Errorf("invalid TMDB Read Access Token")
	}
	if resp.StatusCode == 404 {
		return fmt.Errorf("not found")
	}
	if resp.StatusCode != 200 {
		return fmt.Errorf("TMDB returned status %d", resp.StatusCode)
	}

	body, err := io.ReadAll(resp.Body)
	if err != nil {
		return fmt.Errorf("reading response: %w", err)
	}

	return json.Unmarshal(body, out)
}

func (c *Client) SearchTV(query string) ([]TVShow, error) {
	params := url.Values{"query": {query}}
	var result SearchTVResult
	if err := c.get("/search/tv", params, &result); err != nil {
		return nil, err
	}
	return result.Results, nil
}

func (c *Client) SearchMovie(query string, year int) ([]Movie, error) {
	params := url.Values{"query": {query}}
	if year > 0 {
		params.Set("year", fmt.Sprintf("%d", year))
	}
	var result SearchMovieResult
	if err := c.get("/search/movie", params, &result); err != nil {
		return nil, err
	}
	return result.Results, nil
}

func (c *Client) GetTVShow(id int) (*TVShowDetail, error) {
	var detail TVShowDetail
	params := url.Values{"append_to_response": {"external_ids,content_ratings"}}
	if err := c.get(fmt.Sprintf("/tv/%d", id), params, &detail); err != nil {
		return nil, err
	}
	return &detail, nil
}

func (c *Client) GetSeason(showID, season int) (*Season, error) {
	var s Season
	if err := c.get(fmt.Sprintf("/tv/%d/season/%d", showID, season), nil, &s); err != nil {
		return nil, err
	}
	return &s, nil
}

func (c *Client) GetMovie(id int) (*MovieDetail, error) {
	var detail MovieDetail
	params := url.Values{"append_to_response": {"credits,releases,external_ids"}}
	if err := c.get(fmt.Sprintf("/movie/%d", id), params, &detail); err != nil {
		return nil, err
	}
	return &detail, nil
}

func (c *Client) GetEpisodeGroups(showID int) ([]EpisodeGroupSummary, error) {
	var result EpisodeGroupList
	if err := c.get(fmt.Sprintf("/tv/%d/episode_groups", showID), nil, &result); err != nil {
		return nil, err
	}
	return result.Results, nil
}

func (c *Client) GetEpisodeGroup(groupID string) (*EpisodeGroup, error) {
	var group EpisodeGroup
	if err := c.get(fmt.Sprintf("/tv/episode_group/%s", groupID), nil, &group); err != nil {
		return nil, err
	}
	return &group, nil
}

// ImageURL returns the full URL for an image path.
func ImageURL(path string) string {
	if path == "" {
		return ""
	}
	return imageBase + path
}
