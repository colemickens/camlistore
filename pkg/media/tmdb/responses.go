package tmdb

// TMDB Responses

type tmdbConfig struct {
	Images struct {
		Backdrop_sizes []string `json:"backdrop_sizes"`
		BaseUrl        string   `json:"base_url"` // don't know if this works.
		Poster_sizes   []string `json:"poster_sizes"`
		Profile_sizes  []string `json:"profile_sizes"`
	} `json:"images"`
}

type tmdbMovieResult struct {
	Id int `json:"id"`

	Adult          bool    `json:"adult"`
	Backdrop_path  string  `json:"backdrop_path"`
	Original_title string  `json:"original_title"`
	Release_date   string  `json:"release_date"`
	Poster_path    string  `json:"poster_path"`
	Popularity     float64 `json:"popularity"`
	Title          string  `json:"title"`
}

type tmdbMovieResultPage struct {
	Page          int `json:"page"`
	Total_pages   int `json:"total_pages"`
	Total_results int `json:"total_results"`

	Results []tmdbMovieResult `json:"results"`
}

type tmdbMovieImage struct {
	File_path    string
	Width        int
	Height       int
	Iso_639_1    interface{}
	Aspect_ratio float64
}

type tmdbMovieImages struct {
	id        int              `json:"id"`
	Backdrops []tmdbMovieImage `json:"backdrops"`
	Posters   []tmdbMovieImage `json:"posters"`
}

// Response Types

/*type Movie struct {
	Id           int    `json:"id"`
	Title        string `json:"title"`
	Backdrop_url string
	Poster_url   string
	Backdrops    []Image
	Posters      []Image
}*/

type Movie struct {
	Id             int     `json:"id"`
	Adult          bool    `json:"adult"`
	Backdrop_path  string  `json:"backdrop_path"`
	Original_title string  `json:"original_title"`
	Release_date   string  `json:"release_date"`
	Poster_path    string  `json:"poster_path"`
	Popularity     float64 `json:"popularity"`
	Title          string  `json:"title"`
}

type Image struct {
	File_path    string
	Width        int
	Height       int
	Iso_639_1    interface{}
	Aspect_ratio float64
}

/*
func (api *TmdbApi) convertImgPaths(_imgs []tmdbMovieImage) []Image {
	imgs := make([]Image, len(_imgs))
	for i, _img := range _imgs {
		imgs[i] = Image(_img)
		imgs[i].File_path = api.imageMirror + imgs[i].File_path
	}
	return imgs
}
*/
