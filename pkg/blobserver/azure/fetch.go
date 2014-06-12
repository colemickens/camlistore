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
	"io"

	"camlistore.org/pkg/blob"
)

func (sto *azureStorage) Fetch(blob blob.Ref) (file io.ReadCloser, size uint32, err error) {
	if faultGet.FailErr(&err) {
		return
	}

	res, err := sto.azureClient.FileDownload(sto.table, blob.String())
	if err != nil {
		return nil, 0, err
	}

	defer res.Body.Close()

	return res.Body, uint32(res.ContentLength), nil
}
