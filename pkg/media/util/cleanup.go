package util

import (
	"log"
	"regexp"
	"strconv"
	"strings"
)

const videoFileExtensions string = "m4v|3gp|nsv|ts|ty|strm|rm|rmvb|m3u|ifo|mov|qt|divx|xvid|bivx|vob|nrg|pva|wmv|asf|asx|ogm|m2v|avi|bin|dat|dvr-ms|mpg|mpeg|mp4|mkv|avc|vp3|svq3|nuv|viv|fli|flv"

const filenameJunk string = "480p|720p|dvd|1080p|webdl|rip|brrip|readnfo|xvid|BluRay|nHD|extended edition|BRRip|READNFO|XViD-TDP|x264-NhaNc3|extended|bluray|x264-crossbow|UK|(ENG)|DTS|x264-ESiR|x264-BLOW|PublicHD|x264|DTS-HDChina|unrated|BR|QMax|web-dl|sparks|vedett|DVDRiP|YIFY|h264-nogrp|X264-BARC0DE|X264-AMIABLE"

const titleRegexStr string = `(.*?)(` + videoFileExtensions + `|[\{\(\[]?[0-9]{4}).*`

var titleRegex = regexp.MustCompile(titleRegexStr)

func ParseMovieFilename(filename string) (title string, year int) {
	matches := titleRegex.FindStringSubmatch(filename)

	title = ""
	year = 0

	var err error

	if len(matches) > 1 {
		title = matches[1]
		title = strings.Replace(title, "lotr", "lord of the rings", -1)
		title = strings.Replace(title, "dir cut", "", -1)
		title = strings.Replace(title, ".", " ", -1)
		title = strings.Replace(title, "_", " ", -1)
		title = strings.Trim(title, " ")

	}

	if len(matches) >= 2 {
		year, err = strconv.Atoi(matches[2])
		if err != nil {
			log.Println("atoi failed", filename, err)
			return title, -1
		}
	}

	return title, year
}
