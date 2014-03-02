/*
Copyright 2011 Google Inc.

Licensed under the Apache License, Version 2.0 (the "License");
you may not use this file except in compliance with the License.
You may obtain a copy of the License at

     http://www.apache.org/licenses/LICENSE-2.0

Unless required by applicable law or agreed to in writing, software
distributed under the License is distributed on an "AS IS" BASIS,
WITHOUT WARRANTIES OR CONDITIONS OF ANY KIND, either express or implied.
See the License for the specific language governing permissions and
limitations under the License.
*/

// Package client implements a Camlistore client.
package client

import (
	"bytes"
	"crypto/tls"
	"encoding/json"
	"errors"
	"fmt"
	"io"
	"io/ioutil"
	"log"
	"net"
	"net/http"
	"net/url"
	"os"
	"regexp"
	"strings"
	"sync"
	"time"

	"camlistore.org/pkg/auth"
	"camlistore.org/pkg/blob"
	"camlistore.org/pkg/blobserver"
	"camlistore.org/pkg/client/android"
	"camlistore.org/pkg/httputil"
	"camlistore.org/pkg/misc"
	"camlistore.org/pkg/osutil"
	"camlistore.org/pkg/schema"
	"camlistore.org/pkg/search"
	"camlistore.org/pkg/syncutil"
	"camlistore.org/pkg/types/camtypes"
)

// A Client provides access to a Camlistore server.
type Client struct {
	// server is the input from user, pre-discovery.
	// For example "http://foo.com" or "foo.com:1234".
	// It is the responsibility of initPrefix to parse
	// server and set prefix, including doing discovery
	// to figure out what the proper server-declared
	// prefix is.
	server string

	prefixOnce    syncutil.Once // guards init of following 2 fields
	prefixv       string        // URL prefix before "/camli/"
	isSharePrefix bool          // URL is a request for a share blob

	discoOnce      syncutil.Once
	searchRoot     string      // Handler prefix, or "" if none
	downloadHelper string      // or "" if none
	storageGen     string      // storage generation, or "" if not reported
	syncHandlers   []*SyncInfo // "from" and "to" url prefix for each syncHandler
	serverKeyID    string      // Server's GPG public key ID.

	signerOnce sync.Once
	signer     *schema.Signer
	signerErr  error

	authMode auth.AuthMode
	// authErr is set when no auth config is found but we want to defer warning
	// until discovery fails.
	authErr error

	httpClient *http.Client
	haveCache  HaveCache

	// If sto is set, it's used before the httpClient or other network operations.
	sto blobserver.Storage

	initTrustedCertsOnce sync.Once
	// We define a certificate fingerprint as the 20 digits lowercase prefix
	// of the SHA256 of the complete certificate (in ASN.1 DER encoding).
	// trustedCerts contains the fingerprints of the self-signed
	// certificates we trust.
	// If not empty, (and if using TLS) the full x509 verification is
	// disabled, and we instead check the server's certificate against
	// that list.
	// The camlistore server prints the fingerprint to add to the config
	// when starting.
	trustedCerts []string
	// if set, we also skip the check against trustedCerts
	InsecureTLS bool // TODO: hide this. add accessor?

	initIgnoredFilesOnce sync.Once
	// list of files that camput should ignore.
	// Defaults to empty, but camput init creates a config with a non
	// empty list.
	// See IsIgnoredFile for the matching rules.
	ignoredFiles  []string
	ignoreChecker func(path string) bool

	pendStatMu sync.Mutex             // guards pendStat
	pendStat   map[blob.Ref][]statReq // blobref -> reqs; for next batch(es)

	initSignerPublicKeyBlobrefOnce sync.Once
	signerPublicKeyRef             blob.Ref
	publicKeyArmored               string

	statsMutex sync.Mutex
	stats      Stats

	// via maps the access path from a share root to a desired target.
	// It is non-nil when in "sharing" mode, where the Client is fetching
	// a share.
	via map[string]string // target => via (target is referenced from via)

	log      *log.Logger // not nil
	httpGate *syncutil.Gate
}

const maxParallelHTTP = 5

// New returns a new Camlistore Client.
// The provided server is either "host:port" (assumed http, not https) or a URL prefix, with or without a path, or a server alias from the client configuration file. A server alias should not be confused with a hostname, therefore it cannot contain any colon or period.
// Errors are not returned until subsequent operations.
func New(server string) *Client {
	if !isURLOrHostPort(server) {
		configOnce.Do(parseConfig)
		serverConf, ok := config.Servers[server]
		if !ok {
			log.Fatalf("%q looks like a server alias, but no such alias found in config at %v", server, osutil.UserClientConfigPath())
		}
		server = serverConf.Server
	}
	httpClient := &http.Client{
		Transport: &http.Transport{
			MaxIdleConnsPerHost: maxParallelHTTP,
		},
	}
	return &Client{
		server:     server,
		httpClient: httpClient,
		httpGate:   syncutil.NewGate(maxParallelHTTP),
		haveCache:  noHaveCache{},
		log:        log.New(os.Stderr, "", log.Ldate|log.Ltime),
		authMode:   auth.None{},
	}
}

func NewOrFail() *Client {
	c := New(serverOrDie())
	err := c.SetupAuth()
	if err != nil {
		log.Fatal(err)
	}
	return c
}

// NewStorageClient returns a Client that doesn't use HTTP, but uses s
// directly. This exists mainly so all the convenience methods on
// Client (e.g. the Upload variants) are available against storage
// directly.
// When using NewStorageClient, callers should call Close when done,
// in case the storage wishes to do a cleaner shutdown.
func NewStorageClient(s blobserver.Storage) *Client {
	return &Client{
		sto:       s,
		log:       log.New(os.Stderr, "", log.Ldate|log.Ltime),
		haveCache: noHaveCache{},
	}
}

// TransportConfig contains options for SetupTransport.
type TransportConfig struct {
	// Proxy optionally specifies the Proxy for the transport. Useful with
	// camput for debugging even localhost requests.
	Proxy   func(*http.Request) (*url.URL, error)
	Verbose bool // Verbose enables verbose logging of HTTP requests.
}

// TransportForConfig returns a transport for the client, setting the correct
// Proxy, Dial, and TLSClientConfig if needed. It does not mutate c.
// It is the caller's responsibility to then use that transport to set
// the client's httpClient with SetHTTPClient.
func (c *Client) TransportForConfig(tc *TransportConfig) http.RoundTripper {
	if c == nil {
		return nil
	}
	tlsConfig, err := c.TLSConfig()
	if err != nil {
		log.Fatalf("Error while configuring TLS for client: %v", err)
	}
	var transport http.RoundTripper
	proxy := http.ProxyFromEnvironment
	if tc != nil && tc.Proxy != nil {
		proxy = tc.Proxy
	}
	transport = &http.Transport{
		Dial:                c.DialFunc(),
		TLSClientConfig:     tlsConfig,
		Proxy:               proxy,
		MaxIdleConnsPerHost: maxParallelHTTP,
	}
	httpStats := &httputil.StatsTransport{
		Transport: transport,
	}
	if tc != nil {
		httpStats.VerboseLog = tc.Verbose
	}
	transport = httpStats
	if android.IsChild() {
		transport = &android.StatsTransport{transport}
	}
	return transport
}

type ClientOption interface {
	modifyClient(*Client)
}

func OptionInsecure(v bool) ClientOption {
	return optionInsecure(v)
}

type optionInsecure bool

func (o optionInsecure) modifyClient(c *Client) {
	c.InsecureTLS = bool(o)
}

func OptionTrustedCert(cert string) ClientOption {
	return optionTrustedCert(cert)
}

type optionTrustedCert string

func (o optionTrustedCert) modifyClient(c *Client) {
	cert := string(o)
	if cert != "" {
		c.initTrustedCertsOnce.Do(func() {})
		c.trustedCerts = []string{string(o)}
	}
}

// noop is for use with syncutil.Onces.
func noop() error { return nil }

var shareURLRx = regexp.MustCompile(`^(.+)/(` + blob.Pattern + ")$")

// NewFromShareRoot uses shareBlobURL to set up and return a client that
// will be used to fetch shared blobs.
func NewFromShareRoot(shareBlobURL string, opts ...ClientOption) (c *Client, target blob.Ref, err error) {
	var root string
	m := shareURLRx.FindStringSubmatch(shareBlobURL)
	if m == nil {
		return nil, blob.Ref{}, fmt.Errorf("Unkown share URL base")
	}
	c = New(m[1])
	c.discoOnce.Do(noop)
	c.prefixOnce.Do(noop)
	c.prefixv = m[1]
	c.isSharePrefix = true
	c.authMode = auth.None{}
	c.via = make(map[string]string)
	root = m[2]

	for _, v := range opts {
		v.modifyClient(c)
	}
	c.SetHTTPClient(&http.Client{Transport: c.TransportForConfig(nil)})

	req := c.newRequest("GET", shareBlobURL, nil)
	res, err := c.expect2XX(req)
	if err != nil {
		return nil, blob.Ref{}, fmt.Errorf("Error fetching %s: %v", shareBlobURL, err)
	}
	defer res.Body.Close()
	b, err := schema.BlobFromReader(blob.ParseOrZero(root), res.Body)
	if err != nil {
		return nil, blob.Ref{}, fmt.Errorf("Error parsing JSON from %s: %v", shareBlobURL, err)
	}
	if b.ShareAuthType() != schema.ShareHaveRef {
		return nil, blob.Ref{}, fmt.Errorf("Unknown share authType of %q", b.ShareAuthType())
	}
	target = b.ShareTarget()
	if !target.Valid() {
		return nil, blob.Ref{}, fmt.Errorf("No target.")
	}
	c.via[target.String()] = root
	return c, target, nil
}

// SetHTTPClient sets the Camlistore client's HTTP client.
// If nil, the default HTTP client is used.
func (c *Client) SetHTTPClient(client *http.Client) {
	if client == nil {
		client = http.DefaultClient
	}
	c.httpClient = client
}

// HTTPClient returns the Client's underlying http.Client.
func (c *Client) HTTPClient() *http.Client {
	return c.httpClient
}

// A HaveCache caches whether a remote blobserver has a blob.
type HaveCache interface {
	StatBlobCache(br blob.Ref) (size uint32, ok bool)
	NoteBlobExists(br blob.Ref, size uint32)
}

type noHaveCache struct{}

func (noHaveCache) StatBlobCache(blob.Ref) (uint32, bool) { return 0, false }
func (noHaveCache) NoteBlobExists(blob.Ref, uint32)       {}

func (c *Client) SetHaveCache(cache HaveCache) {
	if cache == nil {
		cache = noHaveCache{}
	}
	c.haveCache = cache
}

func (c *Client) SetLogger(logger *log.Logger) {
	if logger == nil {
		c.log = log.New(ioutil.Discard, "", 0)
	} else {
		c.log = logger
	}
}

func (c *Client) Stats() Stats {
	c.statsMutex.Lock()
	defer c.statsMutex.Unlock()
	return c.stats // copy
}

// ErrNoSearchRoot is returned by SearchRoot if the server doesn't support search.
var ErrNoSearchRoot = errors.New("client: server doesn't support search")

// ErrNoSigning is returned by ServerKeyID if the server doesn't support signing.
var ErrNoSigning = fmt.Errorf("client: server doesn't support signing")

// ErrNoStorageGeneration is returned by StorageGeneration if the
// server doesn't report a storage generation value.
var ErrNoStorageGeneration = errors.New("client: server doesn't report a storage generation")

// ErrNoSync is returned by SyncHandlers if the server does not advertise syncs.
var ErrNoSync = errors.New("client: server has no sync handlers")

// BlobRoot returns the server's blobroot URL prefix.
// If the client was constructed with an explicit path,
// that path is used. Otherwise the server's
// default advertised blobRoot is used.
func (c *Client) BlobRoot() (string, error) {
	prefix, err := c.prefix()
	if err != nil {
		return "", err
	}
	return prefix + "/", nil
}

// ServerKeyID returns the server's GPG public key ID.
// If the server isn't running a sign handler, the error will be ErrNoSigning.
func (c *Client) ServerKeyID() (string, error) {
	if err := c.condDiscovery(); err != nil {
		return "", err
	}
	if c.serverKeyID == "" {
		return "", ErrNoSigning
	}
	return c.serverKeyID, nil
}

// SearchRoot returns the server's search handler.
// If the server isn't running an index and search handler, the error
// will be ErrNoSearchRoot.
func (c *Client) SearchRoot() (string, error) {
	if err := c.condDiscovery(); err != nil {
		return "", err
	}
	if c.searchRoot == "" {
		return "", ErrNoSearchRoot
	}
	return c.searchRoot, nil
}

// StorageGeneration returns the server's unique ID for its storage
// generation, reset whenever storage is reset, moved, or partially
// lost.
//
// This is a value that can be used in client cache keys to add
// certainty that they're talking to the same instance as previously.
//
// If the server doesn't return such a value, the error will be
// ErrNoStorageGeneration.
func (c *Client) StorageGeneration() (string, error) {
	if err := c.condDiscovery(); err != nil {
		return "", err
	}
	if c.storageGen == "" {
		return "", ErrNoStorageGeneration
	}
	return c.storageGen, nil
}

// SyncInfo holds the data that were acquired with a discovery
// and that are relevant to a syncHandler.
type SyncInfo struct {
	From    string
	To      string
	ToIndex bool // whether this sync is from a blob storage to an index
}

// SyncHandlers returns the server's sync handlers "from" and
// "to" prefix URLs.
// If the server isn't running any sync handler, the error
// will be ErrNoSync.
func (c *Client) SyncHandlers() ([]*SyncInfo, error) {
	if err := c.condDiscovery(); err != nil {
		return nil, err
	}
	if c.syncHandlers == nil {
		return nil, ErrNoSync
	}
	return c.syncHandlers, nil
}

var _ search.IGetRecentPermanodes = (*Client)(nil)

// GetRecentPermanodes implements search.IGetRecentPermanodes against a remote server over HTTP.
func (c *Client) GetRecentPermanodes(req *search.RecentRequest) (*search.RecentResponse, error) {
	sr, err := c.SearchRoot()
	if err != nil {
		return nil, err
	}
	url := sr + req.URLSuffix()
	hreq := c.newRequest("GET", url)
	hres, err := c.expect2XX(hreq)
	if err != nil {
		return nil, err
	}
	res := new(search.RecentResponse)
	if err := httputil.DecodeJSON(hres, res); err != nil {
		return nil, err
	}
	if err := res.Err(); err != nil {
		return nil, err
	}
	return res, nil
}

func (c *Client) GetPermanodesWithAttr(req *search.WithAttrRequest) (*search.WithAttrResponse, error) {
	sr, err := c.SearchRoot()
	if err != nil {
		return nil, err
	}
	url := sr + req.URLSuffix()
	hreq := c.newRequest("GET", url)
	hres, err := c.expect2XX(hreq)
	if err != nil {
		return nil, err
	}
	res := new(search.WithAttrResponse)
	if err := httputil.DecodeJSON(hres, res); err != nil {
		return nil, err
	}
	if err := res.Err(); err != nil {
		return nil, err
	}
	return res, nil
}

func (c *Client) Describe(req *search.DescribeRequest) (*search.DescribeResponse, error) {
	sr, err := c.SearchRoot()
	if err != nil {
		return nil, err
	}
	url := sr + req.URLSuffix()
	hreq := c.newRequest("GET", url)
	hres, err := c.expect2XX(hreq)
	if err != nil {
		return nil, err
	}
	res := new(search.DescribeResponse)
	if err := httputil.DecodeJSON(hres, res); err != nil {
		return nil, err
	}
	return res, nil
}

func (c *Client) GetClaims(req *search.ClaimsRequest) (*search.ClaimsResponse, error) {
	sr, err := c.SearchRoot()
	if err != nil {
		return nil, err
	}
	url := sr + req.URLSuffix()
	hreq := c.newRequest("GET", url)
	hres, err := c.expect2XX(hreq)
	if err != nil {
		return nil, err
	}
	res := new(search.ClaimsResponse)
	if err := httputil.DecodeJSON(hres, res); err != nil {
		return nil, err
	}
	return res, nil
}

func (c *Client) Search(req *search.SearchQuery) (*search.SearchResult, error) {
	sr, err := c.SearchRoot()
	if err != nil {
		return nil, err
	}
	url := sr + req.URLSuffix()
	body, err := json.MarshalIndent(req, "", "\t")
	if err != nil {
		return nil, err
	}
	hreq := c.newRequest("POST", url, bytes.NewReader(body))
	hres, err := c.expect2XX(hreq)
	if err != nil {
		return nil, err
	}
	res := new(search.SearchResult)
	if err := httputil.DecodeJSON(hres, res); err != nil {
		return nil, err
	}
	return res, nil
}

// SearchExistingFileSchema does a search query looking for an
// existing file with entire contents of wholeRef, then does a HEAD
// request to verify the file still exists on the server.  If so,
// it returns that file schema's blobref.
//
// May return (zero, nil) on ENOENT. A non-nil error is only returned
// if there were problems searching.
func (c *Client) SearchExistingFileSchema(wholeRef blob.Ref) (blob.Ref, error) {
	sr, err := c.SearchRoot()
	if err != nil {
		return blob.Ref{}, err
	}
	url := sr + "camli/search/files?wholedigest=" + wholeRef.String()
	req := c.newRequest("GET", url)
	res, err := c.doReqGated(req)
	if err != nil {
		return blob.Ref{}, err
	}
	if res.StatusCode != 200 {
		body, _ := ioutil.ReadAll(io.LimitReader(res.Body, 1<<20))
		res.Body.Close()
		return blob.Ref{}, fmt.Errorf("client: got status code %d from URL %s; body %s", res.StatusCode, url, body)
	}
	var ress struct {
		Files []blob.Ref `json:"files"`
	}
	if err := httputil.DecodeJSON(res, &ress); err != nil {
		return blob.Ref{}, fmt.Errorf("client: error parsing JSON from URL %s: %v", url, err)
	}
	if len(ress.Files) == 0 {
		return blob.Ref{}, nil
	}
	for _, f := range ress.Files {
		if c.FileHasContents(f, wholeRef) {
			return f, nil
		}
	}
	return blob.Ref{}, nil
}

// FileHasContents returns true iff f refers to a "file" or "bytes" schema blob,
// the server is configured with a "download helper", and the server responds
// that all chunks of 'f' are available and match the digest of wholeRef.
func (c *Client) FileHasContents(f, wholeRef blob.Ref) bool {
	if err := c.condDiscovery(); err != nil {
		return false
	}
	if c.downloadHelper == "" {
		return false
	}
	req := c.newRequest("HEAD", c.downloadHelper+f.String()+"/?verifycontents="+wholeRef.String())
	res, err := c.expect2XX(req)
	if err != nil {
		log.Printf("download helper HEAD error: %v", err)
		return false
	}
	defer res.Body.Close()
	return res.Header.Get("X-Camli-Contents") == wholeRef.String()
}

// prefix returns the URL prefix before "/camli/", or before
// the blobref hash in case of a share URL.
// Examples: http://foo.com:3179/bs or http://foo.com:3179/share
func (c *Client) prefix() (string, error) {
	if err := c.prefixOnce.Do(c.initPrefix); err != nil {
		return "", err
	}
	return c.prefixv, nil
}

// blobPrefix returns the URL prefix before the blobref hash.
// Example: http://foo.com:3179/bs/camli or http://foo.com:3179/share
func (c *Client) blobPrefix() (string, error) {
	pfx, err := c.prefix()
	if err != nil {
		return "", err
	}
	if !c.isSharePrefix {
		pfx += "/camli"
	}
	return pfx, nil
}

// discoRoot returns the user defined server for this client. It prepends "https://" if no scheme was specified.
func (c *Client) discoRoot() string {
	s := c.server
	if !strings.HasPrefix(s, "http") {
		s = "https://" + s
	}
	return s
}

// initPrefix uses the user provided server URL to define the URL
// prefix to the blobserver root. If the server URL has a path
// component then it is directly used, otherwise the blobRoot
// from the discovery is used as the path.
func (c *Client) initPrefix() error {
	c.isSharePrefix = false
	root := c.discoRoot()
	u, err := url.Parse(root)
	if err != nil {
		return err
	}
	if len(u.Path) > 1 {
		c.prefixv = strings.TrimRight(root, "/")
		return nil
	}
	return c.condDiscovery()
}

func (c *Client) condDiscovery() error {
	if c.sto != nil {
		return errors.New("client not using HTTP")
	}
	return c.discoOnce.Do(c.doDiscovery)
}

// DiscoveryDoc returns the server's JSON discovery document.
// This method exists purely for the "camtool discovery" command.
// Clients shouldn't have to parse this themselves.
func (c *Client) DiscoveryDoc() (io.Reader, error) {
	res, err := c.discoveryResp()
	if err != nil {
		return nil, err
	}
	defer res.Body.Close()
	const maxSize = 1 << 20
	all, err := ioutil.ReadAll(io.LimitReader(res.Body, maxSize+1))
	if err != nil {
		return nil, err
	}
	if len(all) > maxSize {
		return nil, errors.New("discovery document oddly large")
	}
	if len(all) > 0 && all[len(all)-1] != '\n' {
		all = append(all, '\n')
	}
	return bytes.NewReader(all), err
}

func (c *Client) discoveryResp() (*http.Response, error) {
	// If the path is just "" or "/", do discovery against
	// the URL to see which path we should actually use.
	req := c.newRequest("GET", c.discoRoot(), nil)
	req.Header.Set("Accept", "text/x-camli-configuration")
	res, err := c.doReqGated(req)
	if err != nil {
		return nil, err
	}
	if res.StatusCode != 200 {
		res.Body.Close()
		errMsg := fmt.Sprintf("got status %q from blobserver URL %q during configuration discovery", res.Status, c.discoRoot())
		if res.StatusCode == 401 && c.authErr != nil {
			errMsg = fmt.Sprintf("%v. %v", c.authErr, errMsg)
		}
		return nil, errors.New(errMsg)
	}
	// TODO(bradfitz): little weird in retrospect that we request
	// text/x-camli-configuration and expect to get back
	// text/javascript.  Make them consistent.
	if ct := res.Header.Get("Content-Type"); ct != "text/javascript" {
		res.Body.Close()
		return nil, fmt.Errorf("Blobserver returned unexpected type %q from discovery", ct)
	}
	return res, nil
}

func (c *Client) doDiscovery() error {
	root, err := url.Parse(c.discoRoot())
	if err != nil {
		return err
	}

	res, err := c.discoveryResp()
	if err != nil {
		return err
	}

	// TODO: make a proper struct type for this in another package somewhere:
	m := make(map[string]interface{})
	if err := httputil.DecodeJSON(res, &m); err != nil {
		return err
	}

	searchRoot, ok := m["searchRoot"].(string)
	if ok {
		u, err := root.Parse(searchRoot)
		if err != nil {
			return fmt.Errorf("client: invalid searchRoot %q; failed to resolve", searchRoot)
		}
		c.searchRoot = u.String()
	}

	downloadHelper, ok := m["downloadHelper"].(string)
	if ok {
		u, err := root.Parse(downloadHelper)
		if err != nil {
			return fmt.Errorf("client: invalid downloadHelper %q; failed to resolve", downloadHelper)
		}
		c.downloadHelper = u.String()
	}

	c.storageGen, _ = m["storageGeneration"].(string)

	blobRoot, ok := m["blobRoot"].(string)
	if !ok {
		return fmt.Errorf("No blobRoot in config discovery response")
	}
	u, err := root.Parse(blobRoot)
	if err != nil {
		return fmt.Errorf("client: error resolving blobRoot: %v", err)
	}
	c.prefixv = strings.TrimRight(u.String(), "/")

	syncHandlers, ok := m["syncHandlers"].([]interface{})
	if ok {
		for _, v := range syncHandlers {
			vmap := v.(map[string]interface{})
			from := vmap["from"].(string)
			ufrom, err := root.Parse(from)
			if err != nil {
				return fmt.Errorf("client: invalid %q \"from\" sync; failed to resolve", from)
			}
			to := vmap["to"].(string)
			uto, err := root.Parse(to)
			if err != nil {
				return fmt.Errorf("client: invalid %q \"to\" sync; failed to resolve", to)
			}
			toIndex, _ := vmap["toIndex"].(bool)
			c.syncHandlers = append(c.syncHandlers, &SyncInfo{
				From:    ufrom.String(),
				To:      uto.String(),
				ToIndex: toIndex,
			})
		}
	}
	serverSigning, ok := m["signing"].(map[string]interface{})
	if ok {
		c.serverKeyID = serverSigning["publicKeyId"].(string)
	}
	return nil
}

func (c *Client) newRequest(method, url string, body ...io.Reader) *http.Request {
	var bodyR io.Reader
	if len(body) > 0 {
		bodyR = body[0]
	}
	if len(body) > 1 {
		panic("too many body arguments")
	}
	req, err := http.NewRequest(method, c.condRewriteURL(url), bodyR)
	if err != nil {
		panic(err.Error())
	}
	// not done by http.NewRequest in Go 1.0:
	if br, ok := bodyR.(*bytes.Reader); ok {
		req.ContentLength = int64(br.Len())
	}
	c.authMode.AddAuthHeader(req)
	return req
}

// expect2XX will doReqGated and promote HTTP response codes outside of
// the 200-299 range to a non-nil error containing the response body.
func (c *Client) expect2XX(req *http.Request) (*http.Response, error) {
	res, err := c.doReqGated(req)
	if err == nil && (res.StatusCode < 200 || res.StatusCode > 299) {
		buf := new(bytes.Buffer)
		io.CopyN(buf, res.Body, 1<<20)
		res.Body.Close()
		return res, fmt.Errorf("client: got status code %d from URL %s; body %s", res.StatusCode, req.URL.String(), buf.String())
	}
	return res, err
}

func (c *Client) doReqGated(req *http.Request) (*http.Response, error) {
	c.httpGate.Start()
	defer c.httpGate.Done()
	return c.httpClient.Do(req)
}

// insecureTLS returns whether the client is using TLS without any
// verification of the server's cert.
func (c *Client) insecureTLS() bool {
	return c.useTLS() && c.InsecureTLS
}

// selfVerifiedSSL returns whether the client config has fingerprints for
// (self-signed) trusted certificates.
// When true, we run with InsecureSkipVerify and it is our responsibility
// to check the server's cert against our trusted certs.
func (c *Client) selfVerifiedSSL() bool {
	return c.useTLS() && len(c.getTrustedCerts()) > 0
}

// condRewriteURL changes "https://" to "http://" if we are in
// selfVerifiedSSL mode. We need to do that because we do the TLS
// dialing ourselves, and we do not want the http transport layer
// to redo it.
func (c *Client) condRewriteURL(url string) string {
	if c.selfVerifiedSSL() || c.insecureTLS() {
		return strings.Replace(url, "https://", "http://", 1)
	}
	return url
}

// TLSConfig returns the correct tls.Config depending on whether
// SSL is required, the client's config has some trusted certs,
// and we're on android.
func (c *Client) TLSConfig() (*tls.Config, error) {
	if !c.useTLS() {
		return nil, nil
	}
	trustedCerts := c.getTrustedCerts()
	if len(trustedCerts) > 0 {
		return &tls.Config{InsecureSkipVerify: true}, nil
	}
	if !android.OnAndroid() {
		return nil, nil
	}
	return android.TLSConfig()
}

// DialFunc returns the adequate dial function, depending on
// whether SSL is required, the client's config has some trusted
// certs, and we're on android.
// If the client's config has some trusted certs, the server's
// certificate will be checked against those in the config after
// the TLS handshake.
func (c *Client) DialFunc() func(network, addr string) (net.Conn, error) {
	trustedCerts := c.getTrustedCerts()
	if !c.useTLS() || (!c.InsecureTLS && len(trustedCerts) == 0) {
		// No TLS, or TLS with normal/full verification
		if android.IsChild() {
			return func(network, addr string) (net.Conn, error) {
				return android.Dial(network, addr)
			}
		}
		return nil
	}

	return func(network, addr string) (net.Conn, error) {
		var conn *tls.Conn
		var err error
		if android.IsChild() {
			con, err := android.Dial(network, addr)
			if err != nil {
				return nil, err
			}
			conn = tls.Client(con, &tls.Config{InsecureSkipVerify: true})
			if err = conn.Handshake(); err != nil {
				return nil, err
			}
		} else {
			conn, err = tls.Dial(network, addr, &tls.Config{InsecureSkipVerify: true})
			if err != nil {
				return nil, err
			}
		}
		if c.InsecureTLS {
			return conn, nil
		}
		certs := conn.ConnectionState().PeerCertificates
		if certs == nil || len(certs) < 1 {
			return nil, errors.New("Could not get server's certificate from the TLS connection.")
		}
		sig := misc.SHA256Prefix(certs[0].Raw)
		for _, v := range trustedCerts {
			if v == sig {
				return conn, nil
			}
		}
		return nil, fmt.Errorf("Server's certificate %v is not in the trusted list", sig)
	}
}

func (c *Client) Signer() (*schema.Signer, error) {
	c.signerOnce.Do(c.signerInit)
	return c.signer, c.signerErr
}

func (c *Client) signerInit() {
	c.signer, c.signerErr = c.buildSigner()
}

func (c *Client) buildSigner() (*schema.Signer, error) {
	c.initSignerPublicKeyBlobrefOnce.Do(c.initSignerPublicKeyBlobref)
	if !c.signerPublicKeyRef.Valid() {
		return nil, camtypes.Err("client-no-public-key")
	}
	return schema.NewSigner(c.signerPublicKeyRef, strings.NewReader(c.publicKeyArmored), c.SecretRingFile())
}

// sigTime optionally specifies the signature time.
// If zero, the current time is used.
func (c *Client) signBlob(bb schema.Buildable, sigTime time.Time) (string, error) {
	signer, err := c.Signer()
	if err != nil {
		return "", err
	}
	return bb.Builder().SignAt(signer, sigTime)
}

// uploadPublicKey uploads the public key (if one is defined), so
// subsequent (likely synchronous) indexing of uploaded signed blobs
// will have access to the public key to verify it. In the normal
// case, the stat cache prevents this from doing anything anyway.
func (c *Client) uploadPublicKey() error {
	sigRef := c.SignerPublicKeyBlobref()
	if !sigRef.Valid() {
		return nil
	}
	var err error
	if _, keyUploaded := c.haveCache.StatBlobCache(sigRef); !keyUploaded {
		_, err = c.uploadString(c.publicKeyArmored, false)
	}
	return err
}

func (c *Client) UploadAndSignBlob(b schema.AnyBlob) (*PutResult, error) {
	signed, err := c.signBlob(b.Blob(), time.Time{})
	if err != nil {
		return nil, err
	}
	if err := c.uploadPublicKey(); err != nil {
		return nil, err
	}
	return c.uploadString(signed, false)
}

func (c *Client) UploadBlob(b schema.AnyBlob) (*PutResult, error) {
	// TODO(bradfitz): ask the blob for its own blobref, rather
	// than changing the hash function with uploadString?
	return c.uploadString(b.Blob().JSON(), true)
}

func (c *Client) uploadString(s string, stat bool) (*PutResult, error) {
	uh := NewUploadHandleFromString(s)
	uh.SkipStat = !stat
	return c.Upload(uh)
}

func (c *Client) UploadNewPermanode() (*PutResult, error) {
	unsigned := schema.NewUnsignedPermanode()
	return c.UploadAndSignBlob(unsigned)
}

func (c *Client) UploadPlannedPermanode(key string, sigTime time.Time) (*PutResult, error) {
	unsigned := schema.NewPlannedPermanode(key)
	signed, err := c.signBlob(unsigned, sigTime)
	if err != nil {
		return nil, err
	}
	if err := c.uploadPublicKey(); err != nil {
		return nil, err
	}
	return c.uploadString(signed, true)
}

// IsIgnoredFile returns whether the file at fullpath should be ignored by camput.
// The fullpath is checked against the ignoredFiles list, trying the following rules in this order:
// 1) star-suffix style matching (.e.g *.jpg).
// 2) Shell pattern match as done by http://golang.org/pkg/path/filepath/#Match
// 3) If the pattern is an absolute path to a directory, fullpath matches if it is that directory or a child of it.
// 4) If the pattern is a relative path, fullpath matches if it has pattern as a path component (i.e the pattern is a part of fullpath that fits exactly between two path separators).
func (c *Client) IsIgnoredFile(fullpath string) bool {
	c.initIgnoredFilesOnce.Do(c.initIgnoredFiles)
	return c.ignoreChecker(fullpath)
}

// Close closes the client. In most cases, it's not necessary to close a Client.
// The exception is for Clients created using NewStorageClient, where the Storage
// might implement io.Closer.
func (c *Client) Close() error {
	if cl, ok := c.sto.(io.Closer); ok {
		return cl.Close()
	}
	return nil
}
