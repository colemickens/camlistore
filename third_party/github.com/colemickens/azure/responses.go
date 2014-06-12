package azure

import (
	"bytes"
	"encoding/xml"
	"fmt"
	"net/http"
)

type Blobs struct {
	XMLName xml.Name `xml:"EnumerationResults"`
	Items   []Blob   `xml:"Blobs>Blob"`
}

type Blob struct {
	Name     string   `xml:"Name"`
	Property Property `xml:"Properties"`
}

type Property struct {
	LastModified  string `xml:"Last-Modified"`
	Etag          string `xml:"Etag"`
	ContentLength int    `xml:"Content-Length"`
	ContentType   string `xml:"Content-Type"`
	BlobType      string `xml:"BlobType"`
	LeaseStatus   string `xml:"LeaseStatus"`
}

type Error struct {
	Code   int
	Status string
	Body   []byte
	Header http.Header

	AzureCode string
}

func (e *Error) Error() string {
	return fmt.Sprintf("status %d: %s", e.Code, e.Body)
}

func (e *Error) parseXML() {
	var xe xmlError
	_ = xml.NewDecoder(bytes.NewReader(e.Body)).Decode(&xe)
	e.AzureCode = xe.Code
}

type xmlError struct {
	XMLName xml.Name `xml:"Error"`
	Code    string
	Message string
}
