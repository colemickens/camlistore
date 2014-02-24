/*
Copyright 2014 The Camlistore Authors

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

package s3

import (
	"os"
	"testing"

	"camlistore.org/pkg/blobserver"
	"camlistore.org/pkg/blobserver/storagetest"
	"camlistore.org/pkg/jsonconfig"
)

func TestS3(t *testing.T) {
	cfgFile := os.Getenv("CAMLI_S3_TEST_CONFIG_JSON")
	if cfgFile == "" {
		t.Skip("Skipping manual test. To enable, set the environment variable CAMLI_S3_TEST_CONFIG_JSON to the path of a JSON configuration for the s3 storage type.")
	}
	conf, err := jsonconfig.ReadFile(cfgFile)
	if err != nil {
		t.Fatalf("Error reading s3 configuration file %s: %v", cfgFile, err)
	}
	storagetest.Test(t, func(t *testing.T) (sto blobserver.Storage, cleanup func()) {
		sto, err := newFromConfig(nil, conf)
		if err != nil {
			t.Fatalf("newFromConfig error: %v", err)
		}
		return sto, func() {}
	})
}
