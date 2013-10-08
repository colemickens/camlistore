package opensubs

import (
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
