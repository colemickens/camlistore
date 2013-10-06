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
	languages string
	tag       string
	//up        *Uploader
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
	if len(args) != 1 {
		return fmt.Errorf("Must specifiy one media service to lookup against.")
		// TODO: Cmdmain.errorf vs return fmt.errorf?
	}

	subCommand := args[0]
	languages := strings.Split(c.languages, ",")
	_ = languages

	if c.tag == "" {
		return fmt.Errorf("must specify a media tag")
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

	for h, describedBlob := range resp.Meta {
		switch subCommand {
		case "tmdb":
			{
				tmdb, err := tmdb.NewTmdbApi("00ce627bd2e3caf1991f1be7f02fe12c", nil)
				if err != nil {
					return err
				}

				log.Println("---")
				log.Println("hash     ", h)
				if describedBlob.CamliType == "file" {
					log.Println("file     ", describedBlob.File.FileName)
					searchTerm := mediautil.ScrubFilename(describedBlob.File.FileName)
					log.Println("search   ", searchTerm)
					movies := tmdb.LookupMovies(searchTerm)
					if len(movies) > 0 {
						movie := movies[0]
						log.Println("result   ", movie)

						bb1 := schema.NewSetAttributeClaim(describedBlob.BlobRef, "tmdb_title", movie.Title)
						//bb2 := schema.NewSetAttributeClaim(describedBlob.BlobRef, "tmdb_backdrop_url", movie.Backdrop_path)
						//bb3 := schema.NewSetAttributeClaim(describedBlob.BlobRef, "tmdb_poster_url", movie.Poster_path)
						// should we just pull down the backdrop/poster and put it in camlistore as another blob? (yes)

						//for _, bb := range []*schema.Builder{bb1, bb2, bb3} {
						for _, bb := range []*schema.Builder{bb1} {
							log.Println("claim    ", bb)
							put, err := getUploader().UploadAndSignBlob(bb)
							handleResult(bb.Type(), put, err)
						}
					} else {
						log.Println("tmdb failed to find any match")
					}
				}
			}
		case "tvdb":
			{

			}
		case "opensubs":
			{
				//opensubs.CalculateHash()
			}
		case "ffprobe":
			{
				prober, err := ffmpeg.NewProber("ffprobe")
				_ = prober.ProbeFile
				if err != nil {
					return err
				}
			}
		default:
			{
				cmdmain.Errorf("Bad subcommand")
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
