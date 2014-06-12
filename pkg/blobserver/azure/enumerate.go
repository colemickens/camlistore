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

package azure

import (
	"log"

	"camlistore.org/pkg/blob"
	"camlistore.org/pkg/blobserver"
	"camlistore.org/pkg/context"
)

var _ blobserver.MaxEnumerateConfig = (*azureStorage)(nil)

func (sto *azureStorage) MaxEnumerate() int { return 1000 }

func (sto *azureStorage) EnumerateBlobs(ctx *context.Context, dest chan<- blob.SizedRef, after string, limit int) (err error) {
	defer close(dest)
	if faultEnumerate.FailErr(&err) {
		return
	}
	azureblobs, err := sto.azureClient.ListBlobsEx(sto.table, after, limit)
	if err != nil {
		log.Printf("azure ListBucket: %v", err)
		return err
	}
	for _, azureblob := range azureblobs.Items {
		br, ok := blob.Parse(azureblob.Name)
		if !ok {
			continue
		}
		select {
		case dest <- blob.SizedRef{Ref: br, Size: uint32(azureblob.Property.ContentLength)}:
		case <-ctx.Done():
			return context.ErrCanceled
		}
	}
	return nil
}
