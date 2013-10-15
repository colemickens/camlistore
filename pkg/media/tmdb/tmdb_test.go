package tmdb

// This is also kind of a test for mediautil basically

import (
	"log"
	"testing"

	mediautil "camlistore.org/pkg/media/util"
)

func getTmdbApi(t *testing.T) *TmdbApi {
	tmdbApi, err := NewTmdbApi("00ce627bd2e3caf1991f1be7f02fe12c", nil) // insert my test key, whatever or pipe it in?
	if err != nil {
		t.Fatal(err)
	}
	log.Println(tmdbApi.Config)
	return tmdbApi
}

func TestTmdbConfig(t *testing.T) {
	getTmdbApi(t)
}

func TestTmdbSearchMovies(t *testing.T) {

	// extract checkStr() checkInt() into a helper pkg, tired of redoing them repeatedly
}

func TestLookupMovies(t *testing.T) {

	tmdbApi := getTmdbApi(t)

	totalCount := 0
	okCount := 0
	for filename, expectedRes := range expectedResults {
		totalCount++

		search := filename[:len(filename)-4]
		title, year := mediautil.ParseMovieFilename(search)

		if false {
			movie := testLookupMovie(t, tmdbApi, title, year)
			if movie == nil {
				log.Printf("NO TMDB MATCH title(%s) year(%d)\n", title, year)
				continue
			}

			if expectedRes.TmdbId != movie.Id {
				log.Printf("WRONG TMDBID  %s       expected('%d')       got('%d')\n", filename, expectedRes.TmdbId, movie.Id)
				continue
			}
		}
		if expectedRes.Title != title || expectedRes.Year != year {
			log.Printf("PARSE WRONG : %s\n", filename)
			log.Printf("   expected : %s\n", expectedRes.Title)
			log.Printf("     actual : %s\n", title)
			log.Printf("   expected : %d\n", expectedRes.Year)
			log.Printf("     actual : %d\n\n", year)
		} else {
			okCount++
		}
	}
	log.Printf("TOTAL(%d) OK(%d) FAIL(%d) PERCENT(%.2f%%)\n", totalCount, okCount, totalCount-okCount, float64(okCount)/float64(totalCount)*100)

	if totalCount-okCount > 0 {
		t.Fatal("Non-zero failures, see above.")
	}
}

func testLookupMovie(t *testing.T, tmdbApi *TmdbApi, title string, year int) *Movie {
	results := tmdbApi.LookupMovies(title, year)
	if len(results) > 0 {
		res := results[0]
		return &res
	} else {
		return nil
	}
}
