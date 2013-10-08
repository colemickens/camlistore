package opensubs

import (
	"log"
	"os"
	"strconv"
	"testing"
)

func testHashFile(t *testing.T, filename, expected string) {
	f, err := os.Open(filename)
	if err != nil {
		t.Fatal(err)
	}

	hash, err := HashFile(f)
	if err != nil {
		t.Fatal(err)
	}

	shash := strconv.FormatUint(hash, 16)
	if shash != expected {
		t.Fatal("Hash invalid. Got:", shash, "Expected:", expected)
	}
}

func TestHashFiles(t *testing.T) {
	testHashFile(t, "breakdance.avi", "8e245d9679d31e12")
}

func TestLookupMovies(t *testing.T) {
	dir := "/media/data/Media/movies"
	d, err := os.Open(dir)
	if err != nil {
		t.Fatal(err)
	}

	fis, err := d.Readdir(-1)
	if err != nil {
		t.Fatal(err)
	}
	for _, fi := range fis {
		testLookupMovie(t, dir+"/"+fi.Name())
		log.Println()
		log.Println()
		log.Println()
	}
}

func testLookupMovie(t *testing.T, filename string) {
	f, err := os.Open(filename)
	if err != nil {
		t.Fatal(err)
	}

	hash, err := HashFile(f)
	if err != nil {
		t.Fatal(err)
	}

	movie, err := LookupMovieByHash(hash)
	if err != nil {
		t.Fatal(err)
	}

	shash := strconv.FormatUint(hash, 16)
	movieName := ""

	_ = shash
	_ = movie

	log.Println(filename)
	log.Println(movieName)
}
