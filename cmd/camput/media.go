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
 * `devcam put file --filenodes --tag=movie /media/data/Media/oblivion.2013.mp4` [empty file]
 * `devcam put media --tag=movie opensubs`
 * `devcam put media --tag=movie tmdb`
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
	"camlistore.org/pkg/media/opensubs"
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

	// INITIALIZE tmdb/etc here rather than on every single blerb

	for _, v := range resp.WithAttr {
		log.Printf("%+v\n", v)
		//log.Printf(resp.Meta[v.Permanode])
	}

	for h, describedBlob := range resp.Meta {

		log.Println("-----------------------------------")
		log.Printf("%s: %s\n", describedBlob.BlobRef, describedBlob.CamliType)

		pnf, fi, ok := __permanodeFile(describedBlob)
		if ok {
			log.Println(pnf, fi)
		} else {
			// Why is it always in here?
			// Hm
			// I thought I could GetPermanodesWithAttr
			// and then go from the file's permanode to it's PermanodeFile()
			// to get to the FileInfo
			// to then attach attrs to the blob

			// but PermanodeFile(), __permanodeFile() is failing... [see below]

			// (eventually I'll skip this) continue
		}

		switch describedBlob.CamliType {
		case "file":
			log.Printf(" + %v", describedBlob.File)
		case "permanode":
			log.Println(" + %v", describedBlob.Permanode)
		}

		// leaving this to keep playing around with getting as much
		// info about the file blob as I can until I figure out the
		// PermanodeFile() stuff
		if describedBlob.CamliType == "file" {
			switch subCommand {
			case "tmdb":
				tmdb, err := tmdb.NewTmdbApi("00ce627bd2e3caf1991f1be7f02fe12c", nil)
				if err != nil {
					return err
				}

				log.Println("hash     ", h)
				log.Println("file     ", describedBlob.File.FileName)
				searchTerm := mediautil.ScrubFilename(describedBlob.File.FileName)
				log.Println("search   ", searchTerm)
				movies := tmdb.LookupMovies(searchTerm)
				if len(movies) > 0 {
					movie := movies[0]
					log.Println("result   ", movie)

					// should I just pull down the backdrop/poster and put it in camlistore as another blob? (think so)

					for _, bb := range []*schema.Builder{
						schema.NewAddAttributeClaim(describedBlob.BlobRef, "tmdb_title", movie.Title),
						schema.NewSetAttributeClaim(describedBlob.BlobRef, "tmdb_backdrop_url", movie.Backdrop_path),
						schema.NewSetAttributeClaim(describedBlob.BlobRef, "tmdb_poster_url", movie.Poster_path),
					} {
						log.Println("claim    ", bb)
						put, err := getUploader().UploadAndSignBlob(bb)
						handleResult(bb.Type(), put, err)
					}
				} else {
					log.Println("tmdb failed to find any match")
				}

			case "tvdb":
				// tvdb.LookupByFilename()

			case "opensubs":
				_ = opensubs.Hash
				log.Println("opensubs: size:", describedBlob.File.Size)

			case "ffprobe":
				prober, err := ffmpeg.NewProber("ffprobe")
				_ = prober.ProbeFile
				if err != nil {
					return err
				}

			default:
				cmdmain.Errorf("Bad subcommand")

			}
		}
	}

	return nil
}

func __permanodeFile(b *search.DescribedBlob) (path []blob.Ref, fi *search.FileInfo, ok bool) {
	if b == nil || b.Permanode == nil {
		return
	}
	if contentRef := b.Permanode.Attr.Get("camliContent"); contentRef != "" {
		log.Println("b.Request", b.Request) // Why is this always nil?
		cdes := b.Request.DescribedBlobStr(contentRef)
		if cdes != nil && cdes.File != nil {
			return []blob.Ref{b.BlobRef, cdes.BlobRef}, cdes.File, true
		}
	}
	return
}
