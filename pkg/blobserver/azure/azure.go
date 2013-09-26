// +build ignore

/*
Package s3 registers the "s3" blobserver storage type, storing
blobs in an Amazon Web Services' S3 storage bucket.

Example low-level config:

     "/r1/": {
         "handler": "storage-azure",
         "handlerArgs": {
            "container": "books",
            "access_key": "D/akdjflajdlkfjaklsdfj",
            "storage_account": "LibraryProject",
            "skipStartupCheck": false
          }
     },

*/
package azure

import (
	"fmt"
	"net/http"

	"camlistore.org/pkg/blobserver"
	"camlistore.org/pkg/jsonconfig"
	"camlistore.org/pkg/misc/azure/storage"
)

type azureStorage struct {
	azureClient *storage.Client
	container   string
}

func newFromConfig(_ blobserver.Loader, config jsonconfig.Obj) (blobserver.Storage, error) {
	client := &storage.Client{
		Auth:       &azure.Auth{},
		HTTPClient: http.DefaultClient,
	}
	sto := azureStorage{
		azureClient: client,
		container:   config.RequiredString("container"),
	}
	skipStartupCheck := config.OptionalBool("skipStartupCheck", false)
	if err := config.Validate(); err != nil {
		return nil, err
	}

	if !skipStartupCheck {
		// TODO: skip this check if a file
		// ~/.camli/.configcheck/sha1-("IS GOOD: s3: sha1(access key +
		// secret key)") exists and has recent time?
		buckets, err := client.Buckets()
		if err != nil {
			return nil, fmt.Errorf("Failed to get bucket list from S3: %v", err)
		}
		haveBucket := make(map[string]bool)
		for _, b := range buckets {
			haveBucket[b.Name] = true
		}
		if !haveBucket[sto.bucket] {
			return nil, fmt.Errorf("S3 bucket %q doesn't exist. Create it first at https://console.aws.amazon.com/s3/home")
		}
	}
	return sto, nil
}

func init() {
	blobserver.RegisterStorageConstructor("azure", blobserver.StorageConstructor(newFromConfig))
}
