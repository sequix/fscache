# fscache

A filesystem cache in golang.

## Features

* Accessible from multiple processes/threads.
* LRU GC based on atime.
* Provide throughout metrics by struct, you can easily wrap it into Prometheus metrics.
* All functions under one interface, easy to mock.

## Usage

```go
```

## FAQs

1.Will I see a file with half of the content I passed to the cache?

No, as long as the filesystem provide an atomic rename(3) function.

2.Can I set a same key at the same time from three different processes?

You can, but the result is undetermined. In this repo, the same filename will be adapted for the same key,
and the file is accessed exclusively (with LOCK_EX, see flock(2)), so you cannot know who set the last value.
