[![Build Status](https://github.com/sequix/fscache/workflows/main/badge.svg)](https://github.com/sequix/fscache/actions)
[![Go Reference](https://pkg.go.dev/badge/github.com/sequix/fscache.svg)](https://pkg.go.dev/github.com/sequix/fscache)
[![Go Report](https://goreportcard.com/badge/github.com/sequix/fscache)](https://goreportcard.com/report/github.com/sequix/fscache)
[![codecov](https://codecov.io/gh/sequix/fscache/branch/master/graph/badge.svg)](https://codecov.io/gh/sequix/fscache)

# fscache

A filesystem cache in golang.

## Features

* Accessible from multiple threads.
* LRU GC based on atime.
* Provide throughout metrics by struct, you can easily wrap it into Prometheus metrics. (TODO)
* All functions under one interface, easy to mock.

## Usage

```go
package main

import (
	"fmt"
	"io/ioutil"
	"time"

	"github.com/sequix/fscache"
)

func main() {
	cacheDir, err := ioutil.TempDir("", "fscache")
	if err != nil {
		panic(err)
	}
	gcStopCh := make(chan struct{})
	defer close(gcStopCh)

	cache, err := fscache.New(
		fscache.WithCacheDir(cacheDir),
		fscache.WithMaxBytes(10*1024*1024),
		fscache.WithGcInterval(5 * time.Minute),
		fscache.WithGcStopCh(gcStopCh),
	)
	if err != nil {
		panic(err)
	}

	val := []byte("achilles")
	cache.Set("key", val)

	valFromCache, err := cache.Get("key", nil)
	if err != nil {
		panic(err)
	}
	fmt.Println(string(valFromCache))
}
```

## FAQs

1.Will I see a file with half of the content I passed to the cache?

No, as long as the filesystem provide an atomic rename(3) function.

2.Can I use it from multiple processes?

Better not. Because each of the processes will do its own GC.

3.Why there is not a del()?

GC will take care of that. Having a del() will mess the code.

4.Why using []byte not io.Reader/io.Writer?

Personal needs, if you want that, add it yourself.