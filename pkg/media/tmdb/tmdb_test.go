package tmdb

		if false { // nmove this block and the tmdb initialization stuff
			results := tmdbApi.LookupMovies(title, year)
			if len(results) > 0 {
				movie := results[0]
			}

			if movie == nil {
				log.Printf("NO TMDB MATCH title(%s) year(%d)\n", title, year)
				continue
			}

			if expectedRes.TmdbId != movie.Id {
				log.Printf("WRONG TMDBID  %s       expected('%d')       got('%d')\n", filename, expectedRes.TmdbId, movie.Id)
				continue
			}
		}