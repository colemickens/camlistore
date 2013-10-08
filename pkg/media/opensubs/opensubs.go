package opensubs

import (
	"encoding/binary"
	"fmt"
	"os"
)

func Hash(size int64, header, footer []byte) (hash uint64, err error) {
	if len(header) != 8192*8 {
		return 0, fmt.Errorf("not enough header bytes", len(header))
	}

	if len(footer) != 8192*8 {
		return 0, fmt.Errorf("not enough footer bytes", len(footer))
	}

	hash = uint64(size)

	for i := 0; i < 8192; i++ {
		hash += binary.LittleEndian.Uint64(header[i*8 : i*8+8])
	}
	for i := 0; i < 8192; i++ {
		hash += binary.LittleEndian.Uint64(footer[i*8 : i*8+8])
	}

	return hash, nil
}

func HashFile(file *os.File) (hash uint64, err error) {
	fi, err := file.Stat()
	if err != nil {
		return 0, err
	}

	header := make([]byte, 8192*8)
	footer := make([]byte, 8192*8)
	file.Read(header)
	file.Seek(-8192*8, 2)
	file.Read(footer)
	return Hash(fi.Size(), header, footer)
}

func LookupMovieByHash() {

}
