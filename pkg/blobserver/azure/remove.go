// +build ignore

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
	"camlistore.org/pkg/blob"
)

func (sto *azureStorage) RemoveBlobs(blobs []blob.Ref) error {
	// TODO: do these in parallel
	var reterr error
	for _, blob := range blobs {
		if err := sto.azureClient.Delete(sto.bucket, blob.String()); err != nil {
			reterr = err
		}
	}
	return reterr

}
