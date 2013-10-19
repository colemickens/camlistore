package tvdb

import (
	//"camlistore.org/pkg/media/tmdb"

	"testing"
)

func TestTvdbLookup(t *testing.T) {
}

func TestSearchSeriesByName(showName string) {
	serieses, err := tvdbApi.SearchSeriesByName(showName)
	if err != nil {
		panic(err)
	}
	series := serieses[0]

	seriesData, err := tvdbApi.GetSeriesData(series.Id)
	if err != nil {
		panic(err)
	}

	ep := seriesData.E(seasonNumber, episodeNumber)
	if ep == nil {
		log.Println("FAIL", expectedResults)
	} else {
		log.Println("PASS", ep)
	}
}
