package azure

import (
	"bytes"
	"encoding/base64"
	"encoding/xml"
	"fmt"
	"hash"
	"io"
	"io/ioutil"
	"mime"
	"net/http"
	"net/url"
	"path"
	"runtime"
	"strconv"
	"strings"
	"time"
)

const ms_date_layout = "Mon, 02 Jan 2006 15:04:05 GMT"
const version = "2009-09-19"

var client = &http.Client{}

type StorageClient struct {
	Auth      *Auth
	Transport http.RoundTripper
}

func (c *StorageClient) transport() http.RoundTripper {
	if c.Transport != nil {
		return c.Transport
	}
	return http.DefaultTransport
}

func (c *StorageClient) doRequest(req *http.Request) (*http.Response, error) {
	var buf []byte
	runtime.Stack(buf, false)

	copyHeadersToRequest(req, map[string]string{
		"x-ms-date":    time.Now().UTC().Format(ms_date_layout),
		"x-ms-version": version,
	})

	// TODO(colemick): escaping?

	c.Auth.SignRequest(req)

	resp, err := c.transport().RoundTrip(req)
	if err != nil {
		return nil, err
	}

	if resp.StatusCode > 400 && resp.StatusCode < 600 {
		body, _ := ioutil.ReadAll(io.LimitReader(resp.Body, 1<<20))
		aerr := &Error{
			Code:   resp.StatusCode,
			Status: resp.Status,
			Body:   body,
			Header: resp.Header,
		}
		aerr.parseXML()
		resp.Body.Close()
		return nil, aerr
	}

	return resp, nil
}

func NewStorageClient(account, accessKey string) *StorageClient {
	return &StorageClient{
		Auth: &Auth{
			account,
			accessKey,
		},
		Transport: nil,
	}
}

func (c *StorageClient) absUrl(format string, a ...interface{}) string {
	part := fmt.Sprintf(format, a...)
	return fmt.Sprintf("https://%s.blob.core.windows.net/%s", c.Auth.Account, part)
}

func copyHeadersToRequest(req *http.Request, headers map[string]string) {
	for k, v := range headers {
		req.Header[k] = []string{v}
	}
}

func (c *StorageClient) CreateContainer(container string, meta map[string]string) (*http.Response, error) {
	req, err := http.NewRequest(
		"PUT",
		c.absUrl("%s?restype=container", container),
		nil,
	)
	if err != nil {
		return nil, err
	}
	copyHeadersToRequest(req, meta)

	return c.doRequest(req)
}

func (c *StorageClient) DeleteContainer(container string) (*http.Response, error) {
	req, err := http.NewRequest(
		"DELETE",
		c.absUrl("%s?restype=container", container),
		nil,
	)
	if err != nil {
		return nil, err
	}

	return c.doRequest(req)
}

func (c *StorageClient) FileUpload(container, blobName string, body io.Reader) (*http.Response, error) {
	blobName = escape(blobName)
	extension := strings.ToLower(path.Ext(blobName))
	contentType := mime.TypeByExtension(extension)

	req, err := http.NewRequest(
		"PUT",
		c.absUrl("%s?restype=container", container),
		body,
	)
	if err != nil {
		return nil, err
	}
	copyHeadersToRequest(req, map[string]string{
		"x-ms-blob-type": "BlockBlob",
		"Accept-Charset": "UTF-8",
		"Content-Type":   contentType,
		"Content-Length": strconv.FormatInt(req.ContentLength, 10),
	})

	return c.doRequest(req)
}

func (c *StorageClient) FileUploadEx(container, blobName string, body io.Reader, md5h hash.Hash) (*http.Response, error) {
	blobName = escape(blobName)
	extension := strings.ToLower(path.Ext(blobName))
	contentType := mime.TypeByExtension(extension)

	req, err := http.NewRequest(
		"PUT",
		c.absUrl("%s/%s", container, blobName),
		body,
	)
	if err != nil {
		return nil, err
	}
	copyHeadersToRequest(req, map[string]string{
		"x-ms-blob-type": "BlockBlob",
		"Accept-Charset": "UTF-8",
		"Content-Type":   contentType,
		"Content-Length": strconv.FormatInt(req.ContentLength, 10),
	})

	if md5h != nil {
		b64 := new(bytes.Buffer)
		encoder := base64.NewEncoder(base64.StdEncoding, b64)
		encoder.Write(md5h.Sum(nil))
		encoder.Close()

		copyHeadersToRequest(req, map[string]string{
			"Content-MD5": b64.String(),
		})
	}

	return c.doRequest(req)
}

func (c *StorageClient) ListBlobs(container string) (Blobs, error) {
	var blobs Blobs

	req, err := http.NewRequest(
		"GET",
		c.absUrl("%s?restype=container&comp=list", container),
		nil,
	)
	if err != nil {
		return blobs, err
	}

	res, err := c.doRequest(req)
	if err != nil {
		return blobs, err
	}

	defer res.Body.Close()

	decoder := xml.NewDecoder(res.Body)
	decoder.Decode(&blobs)

	return blobs, nil
}

func (c *StorageClient) ListBlobsEx(container string, marker string, maxresults int) (blobs Blobs, err error) {
	req, err := http.NewRequest(
		"GET",
		c.absUrl("%s?restype=container&comp=list&maxresults=%d&marker=%s", container, maxresults, marker),
		nil,
	)
	if err != nil {
		return
	}

	res, err := c.doRequest(req)
	if err != nil {
		return
	}

	defer res.Body.Close()

	decoder := xml.NewDecoder(res.Body)
	decoder.Decode(&blobs)

	return
}

func (c *StorageClient) Stat(container string, blobName string) (size uint32, err error) {
	blobName = escape(blobName)
	req, err := http.NewRequest(
		"HEAD",
		c.absUrl("%s/%s", container, blobName),
		nil,
	)
	if err != nil {
		return 0, err
	}

	res, err := c.doRequest(req)
	if err != nil {
		return 0, err
	}

	defer res.Body.Close()

	return uint32(res.ContentLength), nil
}

func (c *StorageClient) DeleteBlob(container, blobName string) error {
	blobName = escape(blobName)
	req, err := http.NewRequest(
		"DELETE",
		c.absUrl("%s/%s", container, blobName),
		nil,
	)
	if err != nil {
		return err
	}

	res, err := c.doRequest(req)
	if err != nil {
		return err
	}

	defer res.Body.Close()

	if res.StatusCode != 202 {
		body, _ := ioutil.ReadAll(io.LimitReader(res.Body, 1<<20))
		return &Error{
			Code:   res.StatusCode,
			Status: res.Status,
			Body:   body,
			Header: res.Header,
		}
	}

	return nil
}

func (c *StorageClient) FileDownload(container, blobName string) (*http.Response, error) {
	blobName = escape(blobName)
	req, err := http.NewRequest(
		"GET",
		c.absUrl("%s/%s", container, blobName),
		nil,
	)
	if err != nil {
		return nil, err
	}

	return c.doRequest(req)
}

func (c *StorageClient) CopyBlob(container, blobName, source string) (*http.Response, error) {
	source = escape(source)
	blobName = escape(blobName)
	req, err := http.NewRequest(
		"PUT",
		c.absUrl("%s/%s", container, blobName),
		nil,
	)
	if err != nil {
		return nil, err
	}

	copyHeadersToRequest(req, map[string]string{
		"x-ms-copy-source": source,
	})

	return c.doRequest(req)
}

func escape(content string) string {
	content = url.QueryEscape(content)
	// the Azure's behavior uses %20 to represent whitespace instead of + (plus)
	content = strings.Replace(content, "+", "%20", -1)
	// the Azure's behavior uses slash instead of + %2F
	content = strings.Replace(content, "%2F", "/", -1)

	return content
}
