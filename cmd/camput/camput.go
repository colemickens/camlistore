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

package main

import (
	"flag"
	"fmt"
	"log"
	"net/http"
	"net/url"
	"os"
	"strconv"
	"strings"
	"sync"

	"camlistore.org/pkg/client"
	"camlistore.org/pkg/cmdmain"
	"camlistore.org/pkg/httputil"
	"camlistore.org/pkg/syncutil"
)

const buffered = 16 // arbitrary

var (
	flagProxyLocal = false
	flagHTTP       = flag.Bool("verbose_http", false, "show HTTP request summaries")
	flagHaveCache  = true
)

var (
	uploaderOnce sync.Once
	uploader     *Uploader // initialized by getUploader
)

func init() {
	if debug, _ := strconv.ParseBool(os.Getenv("CAMLI_DEBUG")); debug {
		flag.BoolVar(&flagProxyLocal, "proxy_local", false, "If true, the HTTP_PROXY environment is also used for localhost requests. This can be helpful during debugging.")
		flag.BoolVar(&flagHaveCache, "havecache", true, "Use the 'have cache', a cache keeping track of what blobs the remote server should already have from previous uploads.")
	}
	cmdmain.ExtraFlagRegistration = func() {
		client.AddFlags()
	}
	cmdmain.PreExit = func() {
		up := getUploader()
		if up.haveCache != nil {
			up.haveCache.Close()
		}
		stats := up.Stats()
		log.Printf("Client stats: %s", stats.String())
		log.Printf("  #HTTP reqs: %d", up.transport.Requests())
	}
}

func getUploader() *Uploader {
	uploaderOnce.Do(initUploader)
	return uploader
}

func initUploader() {
	up := newUploader()
	if flagHaveCache {
		gen, err := up.StorageGeneration()
		if err != nil {
			log.Printf("WARNING: not using local server inventory cache; failed to retrieve server's storage generation: %v", err)
		} else {
			up.haveCache = NewKvHaveCache(gen)
			up.Client.SetHaveCache(up.haveCache)
		}
	}
	uploader = up
}

func handleResult(what string, pr *client.PutResult, err error) error {
	if err != nil {
		log.Printf("Error putting %s: %s", what, err)
		cmdmain.ExitWithFailure = true
		return err
	}
	fmt.Println(pr.BlobRef.String())
	return nil
}

func getenvEitherCase(k string) string {
	if v := os.Getenv(strings.ToUpper(k)); v != "" {
		return v
	}
	return os.Getenv(strings.ToLower(k))
}

// proxyFromEnvironment is similar to http.ProxyFromEnvironment but it skips
// $NO_PROXY blacklist so it proxies every requests, including localhost
// requests.
func proxyFromEnvironment(req *http.Request) (*url.URL, error) {
	proxy := getenvEitherCase("HTTP_PROXY")
	if proxy == "" {
		return nil, nil
	}
	proxyURL, err := url.Parse(proxy)
	if err != nil || proxyURL.Scheme == "" {
		if u, err := url.Parse("http://" + proxy); err == nil {
			proxyURL = u
			err = nil
		}
	}
	if err != nil {
		return nil, fmt.Errorf("invalid proxy address %q: %v", proxy, err)
	}
	return proxyURL, nil
}

func newUploader() *Uploader {
	cc := client.NewOrFail()
	if !*cmdmain.FlagVerbose {
		cc.SetLogger(nil)
	}

	proxy := http.ProxyFromEnvironment
	if flagProxyLocal {
		proxy = proxyFromEnvironment
	}
	tr := cc.TransportForConfig(
		&client.TransportConfig{
			Proxy:   proxy,
			Verbose: *flagHTTP,
		})
	httpStats, _ := tr.(*httputil.StatsTransport)
	cc.SetHTTPClient(&http.Client{Transport: tr})

	pwd, err := os.Getwd()
	if err != nil {
		log.Fatalf("os.Getwd: %v", err)
	}

	return &Uploader{
		Client:    cc,
		transport: httpStats,
		pwd:       pwd,
		fdGate:    syncutil.NewGate(100), // gate things that waste fds, assuming a low system limit
		fileOpts:  &fileOptions{exifTime: false},
	}
}

func main() {
	cmdmain.Main()
}
