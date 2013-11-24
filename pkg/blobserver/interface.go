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

package blobserver

import (
	"errors"
	"io"
	"net/http"
	"os"
	"time"

	"camlistore.org/pkg/blob"
)

// MaxBlobSize is the size of a single blob in Camlistore.
//
// TODO: formalize this in the specs. This value of 16 MB is less than
// App Engine's 32 MB request limit, much more than Venti's limit, and
// much more than the ~64 KB & 256 KB chunks that the FileWriter make
const MaxBlobSize = 16 << 20

var ErrCorruptBlob = errors.New("corrupt blob; digest doesn't match")

// BlobReceiver is the interface for receiving
type BlobReceiver interface {
	// ReceiveBlob accepts a newly uploaded blob and writes it to
	// permanent storage.
	//
	// Implementations of BlobReceiver downstream of the HTTP
	// server can trust that the source isn't larger than
	// MaxBlobSize and that its digest matches the provided blob
	// ref. (If not, the read of the source will fail before EOF)
	//
	// To ensure those guarantees, callers of ReceiveBlob should
	// not call ReceiveBlob directly but instead use either
	// blobserver.Receive or blobserver.ReceiveString, which also
	// take care of notifying the BlobReceiver's "BlobHub"
	// notification bus for observers.
	ReceiveBlob(br blob.Ref, source io.Reader) (blob.SizedRef, error)
}

type BlobStatter interface {
	// Stat checks for the existence of blobs, writing their sizes
	// (if found back to the dest channel), and returning an error
	// or nil.  Stat() should NOT close the channel.
	// TODO(bradfitz): redefine this to close the channel? Or document
	// better what the synchronization rules are.
	StatBlobs(dest chan<- blob.SizedRef, blobs []blob.Ref) error
}

func StatBlob(bs BlobStatter, br blob.Ref) (sb blob.SizedRef, err error) {
	c := make(chan blob.SizedRef, 1)
	err = bs.StatBlobs(c, []blob.Ref{br})
	if err != nil {
		return
	}
	select {
	case sb = <-c:
	default:
		err = os.ErrNotExist
	}
	return
}

type StatReceiver interface {
	BlobReceiver
	BlobStatter
}

type BlobEnumerator interface {
	// EnumerateBobs sends at most limit SizedBlobRef into dest,
	// sorted, as long as they are lexigraphically greater than
	// after (if provided).
	// limit will be supplied and sanity checked by caller.
	// EnumerateBlobs must close the channel.  (even if limit
	// was hit and more blobs remain)
	EnumerateBlobs(dest chan<- blob.SizedRef,
		after string,
		limit int) error
}

// Cache is the minimal interface expected of a blob cache.
type Cache interface {
	blob.SeekFetcher
	BlobReceiver
	BlobStatter
}

type BlobReceiveConfiger interface {
	BlobReceiver
	Configer
}

type Config struct {
	Writable    bool
	Readable    bool
	Deletable   bool
	CanLongPoll bool

	// the "http://host:port" and optional path (but without trailing slash) to have "/camli/*" appended
	URLBase       string
	HandlerFinder FindHandlerByTyper
}

type BlobRemover interface {
	// RemoveBlobs removes 0 or more blobs.  Removal of
	// non-existent items isn't an error.  Returns failure if any
	// items existed but failed to be deleted.
	RemoveBlobs(blobs []blob.Ref) error
}

// Storage is the interface that must be implemented by a blobserver
// storage type. (e.g. localdisk, s3, encrypt, shard, replica, remote)
type Storage interface {
	blob.StreamingFetcher
	BlobReceiver
	BlobStatter
	BlobEnumerator
	BlobRemover
}

// StorageHandler is a storage implementation that also exports an HTTP
// status page.
type StorageHandler interface {
	Storage
	http.Handler
}

// Optional interface for storage implementations which can be asked
// to shut down cleanly. Regardless, all implementations should
// be able to survive crashes without data loss.
type ShutdownStorage interface {
	Storage
	io.Closer
}

// A GenerationNotSupportedError explains why a Storage
// value implemented the Generationer interface but failed due
// to a wrapped Storage value not implementing the interface.
type GenerationNotSupportedError string

func (s GenerationNotSupportedError) Error() string { return string(s) }

/*
The optional Generationer interface is an optimization and paranoia
facility for clients which can be implemented by Storage
implementations.

If the client sees the same random string in multiple upload sessions,
it assumes that the blobserver still has all the same blobs, and also
it's the same server.  This mechanism is not fundamental to
Camlistore's operation: the client could also check each blob before
uploading, or enumerate all blobs from the server too.  This is purely
an optimization so clients can mix this value into their "is this file
uploaded?" local cache keys.
*/
type Generationer interface {
	// Generation returns a Storage's initialization time and
	// and unique random string (or UUID).  Implementations
	// should call ResetStorageGeneration on demand if no
	// information is known.
	// The error will be of type GenerationNotSupportedError if an underlying
	// storage target doesn't support the Generationer interface.
	StorageGeneration() (initTime time.Time, random string, err error)

	// ResetGeneration deletes the information returned by Generation
	// and re-generates it.
	ResetStorageGeneration() error
}

type Configer interface {
	Config() *Config
}

type StorageConfiger interface {
	Storage
	Configer
}

// MaxEnumerateConfig is an optional interface implemented by Storage
// interfaces to advertise their max value for how many items can
// be enumerated at once.
type MaxEnumerateConfig interface {
	Storage

	// MaxEnumerate returns the max that this storage interface is
	// capable of enumerating at once.
	MaxEnumerate() int
}
