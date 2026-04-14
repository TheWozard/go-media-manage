package tmdb

// Search results

type SearchTVResult struct {
	Page         int      `json:"page"`
	Results      []TVShow `json:"results"`
	TotalResults int      `json:"total_results"`
}

type SearchMovieResult struct {
	Page         int     `json:"page"`
	Results      []Movie `json:"results"`
	TotalResults int     `json:"total_results"`
}

// TV types

type TVShow struct {
	ID           int     `json:"id"`
	Name         string  `json:"name"`
	OriginalName string  `json:"original_name"`
	Overview     string  `json:"overview"`
	FirstAirDate string  `json:"first_air_date"`
	PosterPath   string  `json:"poster_path"`
	BackdropPath string  `json:"backdrop_path"`
	VoteAverage  float64 `json:"vote_average"`
	VoteCount    int     `json:"vote_count"`
}

type TVShowDetail struct {
	TVShow
	Genres            []Genre          `json:"genres"`
	Networks          []Network        `json:"networks"`
	NumberOfSeasons   int              `json:"number_of_seasons"`
	NumberOfEpisodes  int              `json:"number_of_episodes"`
	Seasons           []SeasonSummary  `json:"seasons"`
	Status            string           `json:"status"`
	Tagline           string           `json:"tagline"`
	ExternalIDs       ExternalIDs      `json:"external_ids"`
	ContentRatings    ContentRatings   `json:"content_ratings"`
}

type SeasonSummary struct {
	ID           int    `json:"id"`
	SeasonNumber int    `json:"season_number"`
	Name         string `json:"name"`
	Overview     string `json:"overview"`
	PosterPath   string `json:"poster_path"`
	AirDate      string `json:"air_date"`
	EpisodeCount int    `json:"episode_count"`
}

type Season struct {
	ID           int       `json:"id"`
	SeasonNumber int       `json:"season_number"`
	Name         string    `json:"name"`
	Overview     string    `json:"overview"`
	PosterPath   string    `json:"poster_path"`
	AirDate      string    `json:"air_date"`
	Episodes     []Episode `json:"episodes"`
}

type Episode struct {
	ID             int     `json:"id"`
	Name           string  `json:"name"`
	Overview       string  `json:"overview"`
	SeasonNumber   int     `json:"season_number"`
	EpisodeNumber  int     `json:"episode_number"`
	AirDate        string  `json:"air_date"`
	StillPath      string  `json:"still_path"`
	VoteAverage    float64 `json:"vote_average"`
	VoteCount      int     `json:"vote_count"`
	Runtime        int     `json:"runtime"`
	GuestStars     []Cast  `json:"guest_stars"`
	Crew           []Crew  `json:"crew"`
}

// Movie types

type Movie struct {
	ID           int     `json:"id"`
	Title        string  `json:"title"`
	OriginalTitle string `json:"original_title"`
	Overview     string  `json:"overview"`
	ReleaseDate  string  `json:"release_date"`
	PosterPath   string  `json:"poster_path"`
	BackdropPath string  `json:"backdrop_path"`
	VoteAverage  float64 `json:"vote_average"`
	VoteCount    int     `json:"vote_count"`
}

type MovieDetail struct {
	Movie
	Genres      []Genre     `json:"genres"`
	Runtime     int         `json:"runtime"`
	Status      string      `json:"status"`
	Tagline     string      `json:"tagline"`
	Budget      int64       `json:"budget"`
	Revenue     int64       `json:"revenue"`
	ExternalIDs ExternalIDs `json:"external_ids"`
	Credits     Credits     `json:"credits"`
	Releases    Releases    `json:"releases"`
}

// Shared types

type Genre struct {
	ID   int    `json:"id"`
	Name string `json:"name"`
}

type Network struct {
	ID   int    `json:"id"`
	Name string `json:"name"`
}

type ExternalIDs struct {
	IMDBID    string `json:"imdb_id"`
	TVDBID    int    `json:"tvdb_id"`
}

type ContentRatings struct {
	Results []ContentRating `json:"results"`
}

type ContentRating struct {
	ISO31661 string `json:"iso_3166_1"`
	Rating   string `json:"rating"`
}

type Credits struct {
	Cast []Cast `json:"cast"`
	Crew []Crew `json:"crew"`
}

type Cast struct {
	ID          int    `json:"id"`
	Name        string `json:"name"`
	Character   string `json:"character"`
	ProfilePath string `json:"profile_path"`
	Order       int    `json:"order"`
}

type Crew struct {
	ID          int    `json:"id"`
	Name        string `json:"name"`
	Job         string `json:"job"`
	Department  string `json:"department"`
	ProfilePath string `json:"profile_path"`
}

type Releases struct {
	Countries []Release `json:"countries"`
}

type Release struct {
	ISO31661     string `json:"iso_3166_1"`
	Certification string `json:"certification"`
	ReleaseDate  string `json:"release_date"`
}
