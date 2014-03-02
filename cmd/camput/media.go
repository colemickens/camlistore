/*
Copyright 2011 Google Inc.

Licensed under the Apache License, Version 2.0 (the "License");
you may not use this file except in compliance with the License.
You may obtain a copy of the License at

     http://www.apache.org/licenses/LICENSE-2.0

Unless required by applicable law or agreed to in writing, software
distributed under the License is distributed on an "AS IS" BASIS,
WITHOUT WARRANTIES OR CONDITIONS OF ANY KIND, either express or implied.
See the License for the specific language governing permissions and
limitations under the License.
*/

/*
 * How I'm testing this right now:
 * `rm -rf /tmp/camliroot-${USER} && devcam server`
 * `devcam put file --permanode --tag=movie /media/data/Media/oblivion.2013.mp4` [empty file]
 * `devcam put media --tag=movie`
 */

// TODO: Is it a bug that you can set claims on non-permanodes?

package main

import (
	"flag"
	"fmt"
	"io/ioutil"
	"log"
	"os"
	"strconv"
	"strings"

	"camlistore.org/pkg/blob"
	"camlistore.org/pkg/client"
	"camlistore.org/pkg/cmdmain"
	"camlistore.org/pkg/schema"
	"camlistore.org/pkg/search"

	"camlistore.org/pkg/media/ffmpeg"
	"camlistore.org/pkg/media/opensubs"
	"camlistore.org/pkg/media/tmdb"
	"camlistore.org/pkg/media/tvdb"
	mediautil "camlistore.org/pkg/media/util"
)

type mediaCmd struct {
	fixtitles bool
	languages string
	tag       string

	tmdbApi *tmdb.TmdbApi
	tvdbApi *tvdb.TvdbApi
	prober  *ffmpeg.Prober
	up      *Uploader
}

func init() {
	cmdmain.RegisterCommand("media", func(flags *flag.FlagSet) cmdmain.CommandRunner {
		cmd := new(mediaCmd)
		flags.BoolVar(&cmd.fixtitles, "fixtitles", false, `Fix the title on the file? permanode?`)
		flags.StringVar(&cmd.languages, "languages", "eng", `[type=opensubs] Subtitle languages to download`)
		flags.StringVar(&cmd.tag, "tag", "", `the tag of media to scan`)
		return cmd
	})
}

func (c *mediaCmd) Describe() string {
	return "Add, set, or delete a permanode's attribute."
}

func (c *mediaCmd) Usage() {
	cmdmain.Errorf("Usage: camput [globalopts] media [media_opts] <media_service>")
}

func (c *mediaCmd) Examples() []string {
	return []string{
		"<tag> <type> Lookup [new] items tagged with <tag> against services compatible with <type>", // TODO: FIX
		"media --tag=movie --fixtitles opensubs",
	}
}

func (c *mediaCmd) RunCommand(args []string) error {
	c.up = getUploader()

	languages := strings.Split(c.languages, ",")
	_ = languages

	if c.tag == "" {
		return fmt.Errorf("must specify a media tag")
	}

	var err error

	// initialize client
	c.up = getUploader()

	// initialize tmdb
	c.tmdbApi, err = tmdb.NewTmdbApi("00ce627bd2e3caf1991f1be7f02fe12c", nil)
	if err != nil {
		return err // TODO: make these non-fatal and just skip over them later
	}

	// initialize tvdb
	c.tvdbApi, err = tvdb.NewTvdbApi("78DAA2D23BE41064", nil)
	if err != nil {
		return err
	}

	// initialize ffprobe
	c.prober, err = ffmpeg.NewProber("ffmpeg") // TODO: pipe this in? from env?
	if err != nil {
		return err
	}

	req := &search.WithAttrRequest{
		N:             -1,
		Attr:          "tag",
		Value:         c.tag,
		Fuzzy:         false,
		ThumbnailSize: 0,
	}
	resp, err := c.up.Client.GetPermanodesWithAttr(req)
	if err != nil {
		return err
	}
	for _, wai := range resp.WithAttr {
		var newClaims []*schema.Builder

		dPermaBlob, ok1 := resp.Meta[wai.Permanode.String()]
		dFileBlob, ok2 := permanodeFile(resp.Meta, wai.Permanode)
		if !ok1 || !ok2 {
			continue
		}
		dPermanode := dPermaBlob.Permanode

		if _, present := dPermanode.Attr["tmdb_id"]; !present {
			newClaims = append(newClaims, c.getTmdbClaims(wai.Permanode, dFileBlob)...)
		}

		/*if _, present := dPermanode.Attr["tvdb_episode_id"]; !present {
			newClaims = append(newClaims, c.getTvdbClaims(wai.Permanode, dFileBlob)...)
		}*/

		/*if _, present := dPermanode.Attr["opensubs_id"]; !present {
			newClaims = append(newClaims, c.getOpensubsClaims(wai.Permanode, dFileBlob)...)
		}*/

		/*if _, present := dPermanode.Attr["ffprobe_id"]; !present {
			newClaims = append(newClaims, c.getFfprobeClaims(wai.Permanode, dFileBlob)...)
		}*/

		for _, claim := range newClaims {
			log.Println("claim    ", claim)
			put, err := c.up.Client.UploadAndSignBlob(claim)
			handleResult(claim.Type(), put, err)
		}
	}

	return nil
}

func (c *mediaCmd) getTmdbClaims(permaRef blob.Ref, fileBlob *search.DescribedBlob) (result []*schema.Builder) {
	filename := fileBlob.File.FileName

	title, year := mediautil.ParseMovieFilename(filename)
	//log.Printf("parsed (%s) (%s)\n", title, year)
	movies := c.tmdbApi.LookupMovies(title, year)
	if len(movies) > 0 {
		movie := movies[0]
		log.Println("result   ", movie)

		var imagePutReses [2]*client.PutResult
		for i, imgPath := range []string{movie.Backdrop_path, movie.Poster_path} {
			// attach_file_to_permanode(permanodeRef, attrName, fileBytes)

			imgBytes, err := c.tmdbApi.DownloadImage(imgPath)
			if err != nil {
				panic(err)
			}
			log.Println(imgBytes)

			// download temp file
			f, err := ioutil.TempFile("", "")
			log.Println(f.Name())
			if err != nil {
				// skip
				panic(err)
			}
			n, err := f.Write(imgBytes)
			if n != len(imgBytes) || err != nil {
				panic(err)
			}

			putRes, err := c.up.UploadFile(f.Name())
			if err != nil {
				panic(err)
			}
			log.Printf("%+v", putRes)
			imagePutReses[i] = putRes

			err = os.Remove(f.Name())
			if err != nil {
				panic(err)
			}
			// HELP: do I need to put keep claims on those files to keep 'em from being GC'd?
		}

		result = append(result,
			schema.NewSetAttributeClaim(permaRef, "tmdb_id", strconv.Itoa(movie.Id)),
			schema.NewSetAttributeClaim(permaRef, "tmdb_title", movie.Title),
			//schema.NewSetAttributeClaim(permaRef, "tmdb_backdrop_file", imagePutReses[0].BlobRef.String()),
			//schema.NewSetAttributeClaim(permaRef, "tmdb_poster_file", imagePutReses[1].BlobRef.String()),
			schema.NewSetAttributeClaim(permaRef, "camliPath:tmdb_backdrop", imagePutReses[0].BlobRef.String()),
			schema.NewSetAttributeClaim(permaRef, "camliPath:tmdb_poster", imagePutReses[1].BlobRef.String()),
		)
	} else {
		// log.Println("tmdb failed to find any match")
	}
	return result
}

func (c *mediaCmd) getTvdbClaims(permaRef blob.Ref, fileBlob *search.DescribedBlob) (result []*schema.Builder) {
	filename := fileBlob.File.FileName

	showName, seasonNumber, episodeNumber := mediautil.ParseTvshowFilename(filename)
	if showName == "" || seasonNumber == -1 || episodeNumber == -1 {
		return result
	}

	serieses, err := c.tvdbApi.SearchSeriesByName(showName)
	if err != nil {
		return result
	}
	if len(serieses) > 0 {
		seriesData, err := c.tvdbApi.GetSeriesData(serieses[0].Id)
		if err != nil {
			log.Println("Unexpectedly failed to retrieve tvdb series data for", serieses[0].Id)
		}
		epInfo := seriesData.E(seasonNumber, episodeNumber)
		if epInfo == nil {
			log.Println("failed to retrieve episode info")
			return result
		}
		log.Printf("%+v\n", epInfo)
		result = append(result,
			schema.NewSetAttributeClaim(permaRef, "tvdb_episode_id", strconv.Itoa(epInfo.Id)),
			schema.NewSetAttributeClaim(permaRef, "tvdb_episode_name", epInfo.EpisodeName),
			schema.NewSetAttributeClaim(permaRef, "tvdb_episode_overview", epInfo.Overview),
			schema.NewSetAttributeClaim(permaRef, "tvdb_episode_number", strconv.Itoa(epInfo.EpisodeNumber)),
			schema.NewSetAttributeClaim(permaRef, "tvdb_season_number", strconv.Itoa(epInfo.SeasonNumber)),
			schema.NewSetAttributeClaim(permaRef, "tvdb_series_thumbnail_filref", "?"),
		)
	}
	return result
}

func (c *mediaCmd) getOpensubsClaims(permaRef blob.Ref, fileBlob *search.DescribedBlob) (result []*schema.Builder) {
	size := fileBlob.File.Size

	header_blobs := make([]byte, opensubs.HashChunkSize)
	footer_blobs := make([]byte, opensubs.HashChunkSize)

	client := getUploader().Client;
	blob, size1, err := client.FetchStreaming(fileBlob.BlobRef)
	if err != nil {
		panic(err)
	}

	// assert_eq!(size, size1); // :(

	n, err := blob.Read(header_blobs)
	if err != nil {
		panic(err)
	}

	/*
	ret, err := blob.Seek(-opensubs.HashChunkSize, 2)
	// what is ret?
	if err != nil {
		panic(err)
	}

	n, err = blob.Read(footer_blobs)
	if err != nil {
		panic(err)
	}

	_, _ = n, ret
	*/
	_, _ = n, size1

	blob.Close()

	hash, err := opensubs.Hash(size, header_blobs, footer_blobs)
	if err != nil {
		panic(err)
	}
	movies, err := opensubs.LookupMovieByHash(hash)
	if err != nil {
		panic(err)
	}
	log.Println("******", movies)

	result = append(result)
	return result
}

func (c *mediaCmd) getFfprobeClaims(permaRef blob.Ref, fileBlob *search.DescribedBlob) (result []*schema.Builder) {
	// blerg, feed this into ffmpeg stdin, or figure out a way
	// to look at just the header or something

	_ = c.prober.ProbeFile

	return result
}

func permanodeFile(meta search.MetaMap, permaRef blob.Ref) (*search.DescribedBlob, bool) {
	if fileRef, ok := meta[permaRef.String()].ContentRef(); ok {
		db, ok := meta[fileRef.String()]
		return db, ok
	}
	return nil, false
}
