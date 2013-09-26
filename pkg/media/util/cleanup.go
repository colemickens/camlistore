package util

import (
	"path/filepath"
	"strings"
)

var videoFileExtensions = []string{
	".m4v", ".3gp", ".nsv", ".ts", ".ty", ".strm", ".rm", ".rmvb", ".m3u", ".ifo",
	".mov", ".qt", ".divx", ".xvid", ".bivx", ".vob", ".nrg", ".img", ".iso", ".pva",
	".wmv", ".asf", ".asx", ".ogm", ".m2v", ".avi", ".bin", ".dat", ".dvr-ms", ".mpg",
	".mpeg", ".mp4", ".mkv", ".avc", ".vp3", ".svq3", ".nuv", ".viv", ".fli", ".flv",
	".rar", ".001", ".wpl", ".zip",
}

func isMediaFile(path string) bool {
	for _, v := range videoFileExtensions {
		if filepath.Ext(path) == v {
			return true
		}
	}
	return false
}

// TODO: This, better. Probably with precompiled regex
// do that in init()
// TODO: Do this for... uh... tvshows??
func ScrubFilename(filename string) string {
	for _, v := range videoFileExtensions {
		if filepath.Ext(filename) == v {
			// I think this is intentionally duplicated so that we ensure we don't remove the
			// file extension in the future when we detect better than file extension, but still
			// want to use the filename to assist with lookup
			filename = filename[:len(filename)-(len(v))]
		}
	}
	for _, v := range []string{
		"480p", "720p", "dvd", "1080p", "webdl", "rip",
		"brrip", "readnfo", "xvid", "BluRay", "nHD",
		"extended edition", "BRRip", "READNFO", "XViD-TDP", "x264-NhaNc3",
		"extended", "bluray", "x264-crossbow",
		"UK", "(ENG)",
		"DTS", "x264-ESiR",
		"x264-BLOW", "PublicHD",
		"x264", "DTS-HDChina",
		"unrated", "BR", "QMax",
		// "xvid-{%s}", "x264-{%s}",
	} {
		// case (in)sensitive?
		filename = strings.Replace(filename, v, "", -1)
	}
	filename = strings.Replace(filename, ".", " ", -1)
	filename = strings.Replace(filename, "_", " ", -1)

	filename = strings.Replace(filename, "lotr", "lord of the rings", -1)
	filename = strings.Replace(filename, "SW", "star wars", -1)
	filename = strings.Replace(filename, "dir cut", "", -1)

	return filename
}
