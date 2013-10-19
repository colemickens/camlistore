package util

import (
	/*
		"fmt"
		"os"
		"path/filepath"
	*/
	"testing"
)

func TestParseTvshowFilename(t *testing.T) {
	totalCount := 0
	okCount := 0

	for filename, testTvshow := range tvshowTestData {
		totalCount++

		seriesName, seasonNumber, episodeNumber := ParseTvshowFilename(filename)

		if testTvshow.SeriesName != seriesName || testTvshow.SeasonNumber != seasonNumber || testTvshow.EpisodeNumber != episodeNumber {
			t.Logf("PARSE WRONG : %s\n", filename)
			t.Logf("   expected : %s\n", testTvshow.SeriesName)
			t.Logf("     parsed : %s\n", seriesName)
			t.Logf("   expected : %2d : %2d\n", testTvshow.SeasonNumber, testTvshow.EpisodeNumber)
			t.Logf("     parsed : %2d : %2d\n\n", seasonNumber, episodeNumber)
		} else {
			okCount++
		}
	}
	t.Logf("TOTAL(%d) OK(%d) FAIL(%d) PERCENT(%.2f%%)\n", totalCount, okCount, totalCount-okCount, float64(okCount)/float64(totalCount)*100)

	if totalCount-okCount > 0 {
		t.Fatal("Non-zero failures, see above.")
	}
}

/*
// use this to regenerate test data when we wire up scraping after match rate improves
func TestParseTvshowFile(t *testing.T) {
	filepath.Walk("/media/data/Media/tvshows/", func(path string, info os.FileInfo, err error) error {
		if err != nil {
			panic(err)
		}
		if info.IsDir() {
			return nil
		}
		filename := info.Name()
		suffix := info.Name()[len(info.Name())-3:]
		if suffix == "mkv" || suffix == "avi" || suffix == "mp4" {
			seriesName, seasonNumber, episodeNumber := ParseTvshowFilename(filename)
			fmt.Printf(`"%s": {"%s", %d, %d, -1},`+"\n", filename, seriesName, seasonNumber, episodeNumber)
		}

		return nil
	})
}
*/

func TestParseMovieFilename(t *testing.T) {
	totalCount := 0
	okCount := 0
	for filename, testMovie := range movieTestData {
		totalCount++

		search := filename[:len(filename)-4]
		title, year := ParseMovieFilename(search)

		if testMovie.Title != title || testMovie.Year != year {
			t.Logf("PARSE WRONG : %s\n", filename)
			t.Logf("   expected : %s\n", testMovie.Title)
			t.Logf("     parsed : %s\n", title)
			t.Logf("   expected : %d\n", testMovie.Year)
			t.Logf("     parsed : %d\n\n", year)
		} else {
			okCount++
		}
	}
	t.Logf("TOTAL(%d) OK(%d) FAIL(%d) PERCENT(%.2f%%)\n", totalCount, okCount, totalCount-okCount, float64(okCount)/float64(totalCount)*100)

	if totalCount-okCount > 0 {
		t.Fatal("Non-zero failures, see above.")
	}
}
