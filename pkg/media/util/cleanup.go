package util

import (
	"log"
	"regexp"
	"strconv"
	"strings"
)

const videoFileExtensions string = `mkv|avi|mp4|flv|mov|divx|webm|mpg|vp3|m4v`

const formatTypes string = `dvdscr|dvdrip|dvd|480p|720p|1080p`

const movieRegex1Str string = `(.*) (\d{4}) (?:` + formatTypes + `)`
const movieRegex2Str string = `(.*) (?:` + formatTypes + `)`
const movieRegex3Str string = `(.*) (\d{4})`

var movieRegex1 = regexp.MustCompile(movieRegex1Str)
var movieRegex2 = regexp.MustCompile(movieRegex2Str)
var movieRegex3 = regexp.MustCompile(movieRegex3Str)

func ParseMovieFilename(filename string) (title string, year int) {
	filename = strings.ToLower(filename)
	filename = strings.Replace(filename, "lotr", "lord of the rings", -1)
	filename = strings.Replace(filename, "dir cut", "", -1)
	filename = strings.Replace(filename, ".", " ", -1)
	filename = strings.Replace(filename, "_", " ", -1)

	matches := movieRegex1.FindStringSubmatch(filename)
	if len(matches) == 3 {
		logPrintln(true, "pass regex1", matches[1], ":", matches[2])
		title = strings.Trim(matches[1], " ")
		year, _ = strconv.Atoi(strings.Trim(matches[2], " "))
		return title, year
	} else {
		logPrintln(false, "fail regex1")
	}

	matches = movieRegex2.FindStringSubmatch(filename)
	if len(matches) == 2 {
		logPrintln(true, "pass regex2", matches)
		title = strings.Trim(matches[1], " ")
		return title, -1
	} else {
		logPrintln(false, "fail regex2")
	}

	matches = movieRegex3.FindStringSubmatch(filename)
	if len(matches) == 2 {
		logPrintln(true, "pass regex3", matches)
		title = strings.Trim(matches[1], " ")
		year, _ = strconv.Atoi(strings.Trim(matches[2], " "))
		return title, year
	} else {
		logPrintln(false, "fail regex3")
	}

	return filename, -1
}

func logPrintln(success bool, s ...interface{}) {
	_ = log.Println

	if !success && false {
		log.Println(s...)
	}
}
