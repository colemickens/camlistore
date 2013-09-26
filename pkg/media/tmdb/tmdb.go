package tmdb

import (
	"encoding/json"
	"fmt"
	"net/http"
	"net/url"
)

// create an interface and make a cached version ??
// or like befor where we do it completel diff?

// Tmdb Types
type TmdbApi struct {
	ApiKey      string
	Config      tmdbConfig
	Client      *http.Client
	mirror      string
	imageMirror string
}

func (api *TmdbApi) ImageMirror() string { return api.imageMirror }

func NewTmdbApi(apiKey string, client *http.Client) (*TmdbApi, error) {
	if client == nil {
		client = &http.Client{}
	}
	tmdbApi := &TmdbApi{
		ApiKey:      apiKey,
		Client:      client,
		mirror:      "http://api.themoviedb.org/3",
		imageMirror: "THIS_MUST_BE_REPLACED",
	}

	_url := tmdbApi.mirror + "/configuration"
	err := tmdbApi.get(&tmdbApi.Config, _url, url.Values{})
	if err != nil {
		return nil, err
	}

	return tmdbApi, nil
}

func (tmdbApi *TmdbApi) get(response interface{}, __url string, params url.Values) error {
	// add api key to the params
	// decode into response
	// TODO: seems like there's a better way to construct this url
	params.Set("api_key", tmdbApi.ApiKey)
	_url, err := url.Parse(__url + "?" + params.Encode())
	if err != nil {
		return err
	}

	req, err := http.NewRequest("GET", _url.String(), nil)
	if err != nil {
		return err
	}
	req.Header.Add("Accept", "application/json")

	resp, err := tmdbApi.Client.Do(req)
	if err != nil {
		return err
	}

	//rdr := io.TeeReader(resp.Body, os.Stdout)
	//jsonDec := json.NewDecoder(rdr)

	jsonDec := json.NewDecoder(resp.Body)
	err = jsonDec.Decode(response)
	if err != nil {
		return err
	}

	resp.Body.Close()
	return nil
}

func (t *TmdbApi) LookupMovies(title string) []Movie {
	_url := fmt.Sprintf("%s/search/movie", t.mirror)
	values := url.Values{
		"query": []string{title},
	}

	var results []Movie

	more := true
	for more {

		movieResPage := tmdbMovieResultPage{}
		t.get(&movieResPage, _url, values)
		for _, res := range movieResPage.Results {
			results = append(results, Movie(res))
		}

		if movieResPage.Page >= movieResPage.Total_pages {
			more = false
		}
	}

	// check the page result size, if greater than current page
	// then get then next page

	return results
}
