/*
Copyright 2013 Google Inc.

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

// Package importer imports content from third-party websites.
//
// TODO(bradfitz): Finish this. Barely started.
package importer

import (
	"errors"
	"fmt"
	"log"
	"net/http"
	"sync"

	"camlistore.org/pkg/blobserver"
	"camlistore.org/pkg/jsonconfig"
	"camlistore.org/pkg/search"
)

// A Host is the environment hosting an importer.
type Host struct {
	imp Importer

	// target is the blobserver to populate.
	target blobserver.StatReceiver

	search *search.Handler

	// client optionally specifies how to fetch external network
	// resources.  If nil, http.DefaultClient is used.
	client *http.Client

	mu           sync.Mutex
	running      bool
	stopreq      chan struct{} // closed to signal importer to stop and return an error
	lastProgress *ProgressMessage
	lastRunErr   error
}

func (h *Host) String() string {
	return fmt.Sprintf("%s (a %T)", h.imp.Prefix(), h.imp)
}

func (h *Host) ServeHTTP(w http.ResponseWriter, r *http.Request) {
	switch r.FormValue("mode") {
	case "":
	case "start":
		h.start()
	case "stop":
		h.stop()
	default:
		fmt.Fprintf(w, "Unknown mode")
	}
	h.mu.Lock()
	defer h.mu.Unlock()
	fmt.Fprintf(w, "I am an importer of type %T; running=%v; last progress=%#v",
		h.imp, h.running, h.lastProgress)
}

func (h *Host) start() {
	h.mu.Lock()
	defer h.mu.Unlock()
	if h.running {
		return
	}
	h.running = true
	stopCh := make(chan struct{})
	h.stopreq = stopCh
	go func() {
		log.Printf("Starting importer %s", h)
		err := h.imp.Run(h, stopCh)
		if err != nil {
			log.Printf("Importer %s error: %v", h, err)
		} else {
			log.Printf("Importer %s finished.", h)
		}
		h.mu.Lock()
		defer h.mu.Unlock()
		h.running = false
		h.lastRunErr = err
	}()
}

func (h *Host) stop() {
	h.mu.Lock()
	defer h.mu.Unlock()
	if !h.running {
		return
	}
	h.running = false
	close(h.stopreq)
}

// HTTPClient returns the HTTP client to use.
func (h *Host) HTTPClient() *http.Client {
	if h.client == nil {
		return http.DefaultClient
	}
	return h.client
}

type ProgressMessage struct {
	ItemsDone, ItemsTotal int
	BytesDone, BytesTotal int64
}

// ErrInterrupted should be returned by importers
// when an Interrupt fires.
var ErrInterrupted = errors.New("import interrupted by request")

// An Interrupt is passed to importers for them to monitor
// requests to stop importing.  The channel is closed as
// a signal to stop.
type Interrupt <-chan struct{}

// ShouldStop returns whether the interrupt has fired.
// If so, importers should return ErrInterrupted.
func (i Interrupt) ShouldStop() bool {
	select {
	case <-i:
		return true
	default:
		return false
	}
}

// An Importer imports from a third-party site.
type Importer interface {
	// Run runs a full or increment import.
	Run(*Host, Interrupt) error

	// Prefix returns the unique prefix for this importer.
	// It should be of the form "serviceType:username".
	// Further colons are added to form the names of planned
	// permanodes.
	Prefix() string

	// CanHandleURL returns whether a URL (such as one a user is
	// viewing in their browser and dragged onto Camlistore) is a
	// form recognized by this importer.  If so, its full metadata
	// and full data (e.g. unscaled image) can be fetched, rather
	// than just fetching the HTML of the URL.
	//
	// TODO: implement and use this. For now importers can return
	// stub these and return false/errors. They're unused.
	CanHandleURL(url string) bool
	ImportURL(url string) error
}

type Constructor func(jsonconfig.Obj) (Importer, error)

var (
	mu    sync.Mutex
	ctors = make(map[string]Constructor)
)

func Register(name string, fn Constructor) {
	mu.Lock()
	defer mu.Unlock()
	if _, dup := ctors[name]; dup {
		panic("Dup registration of importer " + name)
	}
	ctors[name] = fn
}

func Create(name string, hl blobserver.Loader, cfg jsonconfig.Obj) (*Host, error) {
	mu.Lock()
	defer mu.Unlock()
	fn := ctors[name]
	if fn == nil {
		return nil, fmt.Errorf("Unknown importer type %q", name)
	}
	imp, err := fn(cfg)
	if err != nil {
		return nil, err
	}
	h := &Host{
		imp: imp,
		// TODO: get search & blobserver from the HandlerLoader
		// via the "root" type.
	}
	return h, nil
}
