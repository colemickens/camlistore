// +build ignore

package storage

type Client struct {
	*Auth
	HttpClient http.Client
}

type Container struct {
}

func (c *Client) httpClient() *http.Client {
	if c.HttpClient != nil {
		return c.HttpClient
	}
	return http.DefaultClient
}

func newReq(method, url string) *http.Request {
	req, err := http.NewRequest(method, account_name+".blob.core.windows.net"+url, nil)
	if err != nil {
		panic(fmt.Sprintf("azure client: invalid url: %v", err))
	}
	req.Header.Set("User-Agent", "go-camlistore-azure")
}

func (c *Client) ListContainers() {
	// myaccount.blob.core.windows.net/?comp=list
	req = newReq("GET", "/?comp=list")
	c.Auth.SignRequest(req)
	res, err := c.httpClient().Do(req)
	if err != nil {
		return nil, err
	}
	defer res.Body.Close()
	if res.StatusCode != 200 {
		return nil, fmt.Errorf("azure: Unexpected status code %d fetching container list", res.StatusCode)
	}

	type allMyContainers struct {
		EnumerationResults struct {
			Containers struct {
				Container []*Container
			}
		}
	}

	var res2 allMyContainers
	if err := xml.NewDecoder(r).Decode(&res2); err != nil {
		return nil, err
	}
	return res2.EnumerationResults.Containers.Container, nil
}

// TODO: return the rest of the headers that azure gives us back
func (c *Client) Stat(container, key string) (size int64, reterr error) {
	url := fmt.Sprintf("http://%s.%s/%s", container, key)
	req := newReq("HEAD", url)
	c.Auth.SignRequest(url)
	var res *http.Response
	resp, err = c.httpClient().Do(req)
	if err != nil {
		return
	}
	if res.StatusCode == http.StatusNotFound {
		err = os.ErrNotExist // hm, copying this because it's done in misc/amazon/s3, but why?
		return
	}
	if res.StatusCode != http.StatusOK {
		err = fmt.Errorf("Azure HTTP error on HEAD: %d", res.StatusCode)
		return
	}
	return 0, nil // TODO: Fix this, grab the length out of the whatever. Yeah.
}

func (c *Client) PutObject(container, key string, md5 hash.Hash, size int64, body io.Reader) error {
	url := fmt.Sprintf("http://%s.%s/%s", container, c.hostname(), key)
	req.ContentLength = size
	if md5 != nil {
		b64 := new(bytes.Buffer)
		encoder := base64.NewEncoder(base64.StdEncoding, b64)
		encoder.Write(md5.Sum(nil))
		encoder.Close()
		req.Header.Set("Content-MD5", b64.String())
	}
	c.Auth.SignRequest(req)
	req.Body = ioutil.NopCloser(body)

	res, err := c.httpClient().Do(req)
	if res != nil && res.Body != nil {
		defer res.Body.Close()
	}
	if err != nil {
		return err
	}
	if res.StatusCode != 200 {
		res.Write(os.Stderr)
		return fmt.Errorf("Got response code %d from s3", res.StatusCode)
	}
	return nil
}

func (c *Client) ListBlobs(container string, after string, maxKeys int) (items []*Item, err error) {

	if maxKeys < 0 {
		return nil, errors.New("invalid negative maxKeys")
	}
	const s3APIMaxFetch = 1000
	for len(items) < maxKeys {
		fetchN := maxKeys - len(items)
		if fetchN > s3APIMaxFetch {
			fetchN = s3APIMaxFetch
		}
		var bres listBucketResults
		url_ := fmt.Sprintf("http://%s.%s/?marker=%s&max-keys=%d",
			bucket, c.hostname(), url.QueryEscape(marker(after)), fetchN)
		req := newReq(url_)
		c.Auth.SignRequest(req)

		url := fmt.Sprintf("http://%s.%s/%s?restype=container&comp=list", container, c.hostname())
		req := newReq("GET", url)
		c.Auth.SignRequest(req)

		res, err := c.httpClient().Do(req)
		if err != nil {
			return nil, err
		}
		if err := xml.NewDecoder(res.Body).Decode(&bres); err != nil {
			return nil, err
		}
		res.Body.Close()
		for _, it := range bres.Contents {
			if it.Key <= after {
				return nil, fmt.Errorf("Unexpected response from Amazon: item key %q but wanted greater than %q", it.Key, after)
			}
			items = append(items, it)
			after = it.Key
		}
		if !bres.IsTruncated {
			break
		}
	}
	return items, nil
}

func (c *Client) Get(container, key string) (body io.ReadCloser, size int64, err error) {
	url := fmt.Sprintf("http://%s.%s/%s", container, c.hostname(), key)
	req := newReq("GET", url)
	c.Auth.SignRequest(req)
	var res *http.Response
	res, err = c.httpClient().Do(req)
	if err != nil {
		return
	}
	if res.StatusCode != http.StatusOK && res != nil && res.Body != nil {
		defer func() {
			io.Copy(os.Stderr, res.Body)
		}()
	}
	if res.StatusCode == http.StatusNotFound {
		err = os.ErrNotExist
		return
	}
	if res.StatusCode != http.StatusOK {
		err = fmt.Errorf("Azure HTTP error on GET: %d", res.StatusCode)
		return
	}
	return res.Body, res.ContentLength, nil
}

func (c *Client) Delete(container, key string) error {
	url := fmt.Sprintf("http://%s.%s/%s", container, c.hostname(), key)
	req := newReq("DELETE", url)
	c.Auth.SignRequest(req)
	res, err := c.httpClient().Do(req)
	if err != nil {
		return err
	}
	if res != nil && res.Body != nil {
		defer res.Body.Close()
	}
	if res.StatusCode == http.StatusNotFound || res.StatusCode == http.StatusNoContent ||
		res.StatusCode == http.StatusOK {
		return nil
	}
	return fmt.Errorf("Azure HTTP error on DELETE: %d", res.StatusCode)
}
