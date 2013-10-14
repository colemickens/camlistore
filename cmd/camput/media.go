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
	mediautil "camlistore.org/pkg/media/util"
)

type mediaCmd struct {
	fixtitles bool
	clean     bool //remove this entirely after dev
	languages string
	tag       string

	tmdbApi *tmdb.TmdbApi
	prober  *ffmpeg.Prober
	up      *Uploader
}

func init() {
	cmdmain.RegisterCommand("media", func(flags *flag.FlagSet) cmdmain.CommandRunner {
		cmd := new(mediaCmd)
		flags.BoolVar(&cmd.fixtitles, "fixtitles", false, `Fix the title on the file? permanode?`)
		flags.BoolVar(&cmd.clean, "clean", false, `Clean removes all potential metadata we've added from all tagged blobs`)
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

	// initialize opensubs
	// TODO

	// initialize ffprobe
	c.prober, err = ffmpeg.NewProber("ffmpeg") // TODO: pipe this in? from env?
	if err != nil {
		return err
	}

	// Look up eligible movie permanodes
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
		log.Println("matched permanode", wai.Permanode)
		var newClaims []*schema.Builder

		dPermaBlob, ok1 := resp.Meta[wai.Permanode.String()]
		dFileBlob, ok2 := permanodeFile(resp.Meta, wai.Permanode)
		if !ok1 || !ok2 {
			continue
		} else {
			log.Println("not skippping")
		}
		dPermanode := dPermaBlob.Permanode

		if !c.clean {
			if _, present := dPermanode.Attr["tmdb_id"]; !present {
				newClaims = append(newClaims, c.getTmdbClaims(wai.Permanode, dFileBlob)...)
			}

			if _, present := dPermanode.Attr["tvdb_id"]; !present {
				newClaims = append(newClaims, c.getTvdbClaims(wai.Permanode, dFileBlob)...)
			}

			if _, present := dPermanode.Attr["opensubs_id"]; !present {
				newClaims = append(newClaims, c.getOpensubsClaims(wai.Permanode, dFileBlob)...)
			}

			if _, present := dPermanode.Attr["ffprobe_id"]; !present {
				newClaims = append(newClaims, c.getFfprobeClaims(wai.Permanode, dFileBlob)...)
			}
		} else {
			log.Println("cleaing attributes for permanode", wai.Permanode)
			for _, attrName := range []string{
				"tmdb_id", "tmdb_title", "tmdb_backdrop_fileurl", "tmdb_poster_file",
			} {
				newClaims = append(newClaims, schema.NewDelAttributeClaim(wai.Permanode, attrName, ""))
			}
		}

		// apply claims
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

	searchTerm := mediautil.ScrubFilename(filename)
	log.Println("search   ", searchTerm)
	movies := c.tmdbApi.LookupMovies(searchTerm)
	if len(movies) > 0 {
		movie := movies[0]
		log.Println("result   ", movie)

		var imagePutReses [2]*client.PutResult
		for i, imgPath := range []string{movie.Backdrop_path, movie.Poster_path} {
			imgBytes, err := c.tmdbApi.DownloadImage(imgPath)
			if err != nil {
				// TODO : handle
				panic(err)
			}
			log.Println(imgBytes)
			// I really don't need the "file" attrs that are getting uploaded
			// "raw file" or something?

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

			/*
				imgBlob, err := schema.BlobFromReader(blob.SHA1FromBytes(imgBytes), bytes.NewBuffer(imgBytes))
				if err != nil {
					// TODO : handle
					panic(err)
				}

				imagePutReses[i], err = c.up.Client.UploadBlob(imgBlob)
				if err != nil {
					// TODO : handle
					panic(err)
				}
			*/

			// HELP: do I need to put keep claims on those files to keep 'em from being GC'd?
		}

		result = append(result,
			schema.NewSetAttributeClaim(permaRef, "tmdb_id", strconv.Itoa(movie.Id)),
			schema.NewSetAttributeClaim(permaRef, "tmdb_title", movie.Title),
			schema.NewSetAttributeClaim(permaRef, "tmdb_backdrop_file", imagePutReses[0].BlobRef.String()),
			schema.NewSetAttributeClaim(permaRef, "tmdb_poster_file", imagePutReses[1].BlobRef.String()),
		)
	} else {
		log.Println("tmdb failed to find any match")
	}
	return result
}

func (c *mediaCmd) getTvdbClaims(permaRef blob.Ref, fileBlob *search.DescribedBlob) []*schema.Builder {
	return []*schema.Builder{}
}

func (c *mediaCmd) getOpensubsClaims(permaRef blob.Ref, fileBlob *search.DescribedBlob) []*schema.Builder {
	_ = opensubs.Hash
	log.Println("opensubs: size:", fileBlob.File.Size)
	// not sure there's a way to get an offset bytes from a file blob?
	return []*schema.Builder{}
}

func (c *mediaCmd) getFfprobeClaims(permaRef blob.Ref, fileBlob *search.DescribedBlob) []*schema.Builder {
	// blerg, feed this into ffmpeg stdin, or figure out a way
	// to look at just the header or something

	_ = c.prober.ProbeFile

	return []*schema.Builder{}
}

func permanodeFile(meta search.MetaMap, permaRef blob.Ref) (*search.DescribedBlob, bool) {
	if fileRef, ok := meta[permaRef.String()].ContentRef(); ok {
		db, ok := meta[fileRef.String()]
		return db, ok
	}
	return nil, false

	/*if !ok {
		panic("TODO: Remove this panic, just want to see if it is EVER hit.")
		fileBlob, ok = c.getFileBlob(fileRef)
		if !ok {
			// skip, there's not a file on the other side...
			continue
		}
	}*/
}
