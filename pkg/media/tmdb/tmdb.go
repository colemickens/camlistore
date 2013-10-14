package tmdb

import (
	"encoding/json"
	"fmt"
	"io/ioutil"
	"net/http"
	"net/url"
	"strconv"
)

// create an interface and make a cached version ??
// or like befor where we do it completel diff?

const imageSize = "original"

// Tmdb Types
type TmdbApi struct {
	ApiKey string
	Config tmdbConfig
	client *http.Client
	mirror string
}

func NewTmdbApi(apiKey string, client *http.Client) (*TmdbApi, error) {
	if client == nil {
		client = &http.Client{}
	}
	tmdbApi := &TmdbApi{
		ApiKey: apiKey,
		client: client,
		mirror: "http://api.themoviedb.org/3",
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

	resp, err := tmdbApi.client.Do(req)
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

func (t *TmdbApi) LookupMovies(title string, year int) []Movie {
	_url := fmt.Sprintf("%s/search/movie", t.mirror)

	values := make(url.Values)
	values.Set("query", title)
	if year != -1 {
		values.Set("year", strconv.Itoa(year))
	}

	var results []Movie

	//maxPages := 2
	more := true
	//for more {

	movieResPage := tmdbMovieResultPage{}
	err := t.get(&movieResPage, _url, values)
	if err != nil {
		panic(err) // TODO: handle this
	}
	for _, res := range movieResPage.Results {
		results = append(results, Movie(res))
	}

	if movieResPage.Page >= movieResPage.Total_pages {
		more = false
		_ = more
	}
	//}
	// TODO: re enable this, have a max pages somewhere
	// too lazy to fix now

	// check the page result size, if greater than current page
	// then get then next page

	return results
}

func (t *TmdbApi) DownloadImage(suffix string) (imageBytes []byte, err error) {
	_url := t.url(t.Config.Images.BaseUrl + imageSize + suffix)

	req, err := http.NewRequest("GET", _url, nil)
	if err != nil {
		return nil, err
	}

	resp, err := t.client.Do(req)
	if err != nil {
		return nil, err
	}
	return ioutil.ReadAll(resp.Body)
}

func (t *TmdbApi) url(path string) string {
	return fmt.Sprintf("%s/", path)
}
