# Azure

Forked from: https://github.com/loldesign/azure with some modifications

A golang API to communicate with the Azure Storage.
For while, only for manager blobs and containers (create, destroy and so on).

## Installation

```go get github.com/colemickens/azure```

## Usage

### Creating a container

```go
package main

import(
  "fmt"
  "github.com/colemickens/azure"
)

func main() {
  blob := azure.New("accountName", "secret")
  res, err := blob.CreateContainer("mycontainer")

  if err != nil {
    fmt.Println(err)
  }

  fmt.Printf("status -> %s", res.Status)
}
```

### Uploading a file to container

```go
package main

import(
  "fmt"
  "github.com/colemickens/azure"
)

func main() {
  blob := azure.New("accountName", "secret")

  file, err := os.Open("path/of/myfile.txt")

  if err != nil {
    fmt.Println(err)
  }

  res, err := blob.FileUpload("mycontainer", "file_name.txt", file)

  if err != nil {
    fmt.Println(err)
  }

  fmt.Printf("status -> %s", res.Status)
}
```

### Listing container's blobs

```go
package main

import(
  "fmt"
  "github.com/colemickens/azure"
)

func main() {
  blob := azure.New("accountName", "secret")
  blobs, err := blob.ListBlobs("mycontainer")

  if err != nil {
    fmt.Println(err)
  }

  for _, file := range blobs.Items {
    fmt.Printf("blob -> %+v", file)
  }
}
```

### Downloading a file from container

```go
package main

import(
  "fmt"
  "github.com/colemickens/azure"
)

func main() {
  blob := azure.New("accountName", "secret")
  res, err := blob.FileDownload("mycontainer", "some/filename.png")

  if err != nil {
    fmt.Println(err)
  }

  contents, ok := ioutil.ReadAll(res.Body)

  if ok != nil {
    fmt.Println(ok)
  }

  ok = ioutil.WriteFile("filename.png"), contents, 0644) // don't do that with large files!

  if ok != nil {
    fmt.Println("done!")
  }
}
```

### Deleting a blob

```go
package main

import(
  "fmt"
  "github.com/colemickens/azure"
)

func main() {
  blob := azure.New("accountName", "secret")
  ok, err := blob.DeleteBlob("mycontainer", "my_file.png")

  if err != nil {
    fmt.Println(err)
  }

  fmt.Printf("deleted? -> %t", ok)
}
```

### Deleting a container

```go
package main

import(
  "fmt"
  "github.com/colemickens/azure"
)

func main() {
  blob := azure.New("accountName", "secret")
  res, err := blob.DeleteContainer("mycontainer")

  if err != nil {
    fmt.Println(err)
  }

  fmt.Printf("status -> %s", res.Status)
}
```

## Contributing

1. Fork it
2. Create your feature branch (`git checkout -b my-new-feature`)
3. Commit your changes (`git commit -am "Added some feature"`)
4. Push to the branch (`git push origin my-new-feature`)
5. Create new Pull Request
