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

/*
Package azure registers the "azure" blobserver storage type, storing
blobs in an Amazon Web Services' azure storage table.

Example low-level config:

     "/r1/": {
         "handler": "storage-azure",
         "handlerArgs": {
            "account_name": "camlidev",
            "table": "camli-test-table-aaa",
            "secret": "...",
            "createTable": false,
            "skipStartupCheck": false
          }
     },

*/
package azure

import (
	"fmt"

	"camlistore.org/pkg/blobserver"
	"camlistore.org/pkg/fault"
	"camlistore.org/pkg/jsonconfig"
	"camlistore.org/third_party/github.com/colemickens/azure"
)

var (
	faultReceive   = fault.NewInjector("azure_receive")
	faultEnumerate = fault.NewInjector("azure_enumerate")
	faultStat      = fault.NewInjector("azure_stat")
	faultGet       = fault.NewInjector("azure_get")
)

type azureStorage struct {
	azureClient *azure.StorageClient
	table       string
	hostname    string
}

func (s *azureStorage) String() string {
	return fmt.Sprintf("\"azure\" blob storage at host %q, table %q", s.hostname, s.table)
}

func newFromConfig(_ blobserver.Loader, config jsonconfig.Obj) (blobserver.Storage, error) {
	sto := &azureStorage{
		azureClient: azure.NewStorageClient(
			config.RequiredString("account_name"),
			config.RequiredString("secret")),
		table: config.RequiredString("table"),
	}

	skipStartupCheck := config.OptionalBool("skipStartupCheck", false)
	createTableOnDemand := config.OptionalBool("createTable", false)
	if err := config.Validate(); err != nil {
		return nil, err
	}

	if !skipStartupCheck {
		_, err := sto.azureClient.ListBlobsEx(sto.table, "", 1)

		if serr, ok := err.(*azure.Error); ok {
			if serr.AzureCode == "ContainerNotFound" {
				if createTableOnDemand {
					resp, err := sto.azureClient.CreateContainer(sto.table, nil)
					defer resp.Body.Close()
					if err != nil {
						return nil, fmt.Errorf("azure: failed to create table %q", sto.table)
					}
				} else {
					return nil, fmt.Errorf("azure: table %q does not exist", sto.table)
				}
			}
		} else if err != nil {
			return nil, fmt.Errorf("azure: failed to list table %q: %v", sto.table, err)
		}
	}
	return sto, nil
}

func init() {
	blobserver.RegisterStorageConstructor("azure", blobserver.StorageConstructor(newFromConfig))
}
