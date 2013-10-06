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

package main

import (
	"flag"
	"fmt"
	"log"
	"strings"

	"camlistore.org/pkg/blob"
	"camlistore.org/pkg/client"
	"camlistore.org/pkg/cmdmain"
	"camlistore.org/pkg/schema"
	"camlistore.org/pkg/search"

	"camlistore.org/pkg/media/ffmpeg"
	"camlistore.org/pkg/media/tmdb"
	mediautil "camlistore.org/pkg/media/util"
)

type mediaCmd struct {
	fixtitles bool
	opensubs  bool
	tmdb      bool
	tvdb      bool
	ffprobe   bool
	languages string
	tag       string
	//up        *Uploader
}

func init() {
	cmdmain.RegisterCommand("media", func(flags *flag.FlagSet) cmdmain.CommandRunner {
		cmd := new(mediaCmd)
		flags.BoolVar(&cmd.fixtitles, "fixtitles", false, `Fix the title on the file? permanode?`)
		flags.StringVar(&cmd.languages, "languages", "eng", `[type=opensubs] Subtitle languages to download`)
		return cmd
	})
}

func (c *mediaCmd) Describe() string {
	return "Add, set, or delete a permanode's attribute."
}

func (c *mediaCmd) Usage() {
	//cmdmain.Errorf("Usage: camput [globalopts] attr [attroption] <permanode> <name> <value>")
	cmdmain.Errorf("Usage: camput [globalopts] media [mediaoption]")
}

func (c *mediaCmd) Examples() []string {
	return []string{
		"<tag> <type> Lookup [new] items tagged with <tag> against services compatible with <type>",
	}
}

func (c *mediaCmd) RunCommand(args []string) error {
	languages := strings.Split(c.languages, ",")
	if c.opensubs && (c.fixtitles == true || len(languages) > 0) {

	}

	if c.tag == "" {
		cmdmain.Errorf("must specify a media tag")
	}

	var err error

	req := &search.WithAttrRequest{
		N:             -1,
		Attr:          "tag",
		Value:         c.tag,
		Fuzzy:         false,
		ThumbnailSize: 0,
	}

	client := client.NewOrFail()
	resp, err := client.GetPermanodesWithAttr(req)
	if err != nil {
		return err
	}

	for h, db := range resp.Meta {
		if c.opensubs {
			//opensubs.CalculateHash()
		}

		if c.tmdb {
			tmdb, err := tmdb.NewTmdbApi("00ce627bd2e3caf1991f1be7f02fe12c", nil)

			log.Println("---")
			log.Println("hash     ", h)
			if db.CamliType == "file" {
				log.Println("file     ", db.File.FileName)
				searchTerm := mediautil.ScrubFilename(db.File.FileName)
				log.Println("tmdb     ", searchTerm)
				movies := tmdb.LookupMovies(searchTerm)
				if len(movies) > 0 {
					log.Println("tmdb 1st ", movies[0])
				} else {
					log.Println("tmdb nada")
				}
			}
			if db.CamliType == "permanode" {
				log.Println("permanode", db.Permanode.Attr["title"])
			}
		}

		if c.ffprobe {
			prober, err := ffmpeg.NewProber("ffprobe")
			_ = prober.ProbeFile
			if err != nil {
				return err
			}
		}
	}

	log.Println("---")

	// add a new job to the job pool
	// to fire off to ffprobe/tmdb/tvdb/etc
	// with funcs to write into new attr claims when done
	// (if they don't exist)
	// namespace tags?
	// do these richer types deserve their own camliType?

	/*
		pn, ok := blob.Parse(permanode)
		if !ok {
			return fmt.Errorf("Error parsing blobref %q", permanode)
		}
		bb := schema.NewSetAttributeClaim(pn, attr, value)
		if c.add {
			if c.del {
				return errors.New("Add and del options are exclusive")
			}
			bb = schema.NewAddAttributeClaim(pn, attr, value)
		} else {
			// TODO: del, which can make <value> be optional
			if c.del {
				return errors.New("del not yet implemented")
			}
		}
		put, err := getUploader().UploadAndSignBlob(bb)
		handleResult(bb.Type(), put, err)
	*/

	_ = fmt.Println
	_ = blob.Parse
	_ = schema.NewSetAttributeClaim

	return nil
}
