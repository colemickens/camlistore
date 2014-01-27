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
	"camlistore.org/pkg/blob"
	"camlistore.org/pkg/context"
)

const buffered = 8

// TODO: it'd be nice to make sources be []BlobEnumerator, but that
// makes callers more complex since assignable interfaces' slice forms
// aren't assignable.
func MergedEnumerate(ctx *context.Context, dest chan<- blob.SizedRef, sources []Storage, after string, limit int) error {
	defer close(dest)

	startEnum := func(source Storage) (*blob.ChanPeeker, <-chan error) {
		ch := make(chan blob.SizedRef, buffered)
		errch := make(chan error, 1)
		go func() {
			errch <- source.EnumerateBlobs(ctx, ch, after, limit)
		}()
		return &blob.ChanPeeker{Ch: ch}, errch
	}

	peekers := make([]*blob.ChanPeeker, 0, len(sources))
	errs := make([]<-chan error, 0, len(sources))
	for _, source := range sources {
		peeker, errch := startEnum(source)
		peekers = append(peekers, peeker)
		errs = append(errs, errch)
	}

	nSent := 0
	lastSent := ""
	for nSent < limit {
		lowestIdx := -1
		var lowest blob.SizedRef
		for idx, peeker := range peekers {
			for !peeker.Closed() && peeker.MustPeek().Ref.String() <= lastSent {
				peeker.Take()
			}
			if peeker.Closed() {
				continue
			}
			sb := peeker.MustPeek()                                       // can't be nil if not Closed
			if lowestIdx == -1 || sb.Ref.String() < lowest.Ref.String() { // TODO: add cheaper Ref comparison function, avoiding String
				lowestIdx = idx
				lowest = sb
			}
		}
		if lowestIdx == -1 {
			// all closed
			break
		}

		dest <- lowest
		nSent++
		lastSent = lowest.Ref.String()
	}

	// Once we've gotten enough, ignore the rest of whatever's
	// coming in.
	for _, peeker := range peekers {
		go peeker.ConsumeAll()
	}

	// If any part returns an error, we return an error.
	var retErr error
	for _, errch := range errs {
		if err := <-errch; err != nil {
			retErr = err
		}
	}
	return retErr
}
