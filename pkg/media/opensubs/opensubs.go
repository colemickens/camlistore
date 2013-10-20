package opensubs

import (
	"encoding/binary"
	"fmt"
	"log"
	"os"
	"strconv"

	"camlistore.org/third_party/github.com/kolo/xmlrpc"
)

const (
	user       = ""
	pass       = ""
	lang       = "eng"
	user_agent = "OS Test User Agent"

	HashChunkSize = 8192 * 8
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

func LookupMovieByHash(hash uint64) (res xmlrpc.Struct, err error) {
	shash := strconv.FormatUint(hash, 16)
	client, err := xmlrpc.NewClient("http://api.opensubtitles.org/xml-rpc", nil)
	if err != nil {
		return nil, err
	}

	result := xmlrpc.Struct{}
	params := xmlrpc.Params{Params: []interface{}{user, pass, lang, user_agent}}
	err = client.Call("LogIn", params, &result)
	if err != nil {
		panic(err)
	}

	// get the token out
	token := result["token"]

	params = xmlrpc.Params{Params: []interface{}{token, []interface{}{shash}}}
	log.Println("CheckMovieHash", params)
	//err = client.Call("CheckMovieHash", params, &result)
	err = client.Call("CheckMovieHash2", params, &result)
	if err != nil {
		panic(err)
	}

	log.Printf("", result)
	log.Printf("", result["data"])
	//movie := result["data"].(xmlrpc.Struct)[shash] // TODO: handle... dupes for a hash, I guess?

	return result, nil
}
