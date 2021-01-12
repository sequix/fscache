package fscache

import (
	"bytes"
	"container/heap"
	"errors"
	"io"
	"io/ioutil"
	"os"
	"path/filepath"
	"sync"
	"syscall"
	"time"

	"github.com/gofrs/flock"
)

// TODO: doc; readme; test

type Interface interface {
	Set(key string, src io.Reader) error
	Get(key string, dst io.Writer) error
	Has(key string) bool
	Del(key string)
	SetBytes(key string, src []byte) error
	GetBytes(key string, dst []byte) ([]byte, error)
}

var (
	ErrNotFound = errors.New("not found")
)

// FsCache is a LRU filesystem cache.
type FsCache struct {
	cacheDir   string
	maxBytes   int64
	curBytes   int64
	gcInterval time.Duration
	fih        fileInfoHeap
	fihMu      sync.RWMutex
	gcStopCh   <-chan struct{}
	gcFlock    *flock.Flock
}

func New(opts ...Option) (Interface, error) {
	fc := &FsCache{
		cacheDir:   os.TempDir(),
		gcInterval: 5 * time.Minute,
	}
	for _, opt := range opts {
		opt(fc)
	}
	if err := fc.init(); err != nil {
		return nil, err
	}
	if fc.maxBytes > 0 {
		fc.gcFlock = flock.New(filepath.Join(fc.cacheDir, "gc.lock"))
		go fc.gcRunner()
	}
	return fc, nil
}

func (fc *FsCache) init() error {
	return filepath.Walk(filepath.Join(fc.cacheDir, "cache"), func(path string, info os.FileInfo, err error) error {
		if err != nil {
			return err
		}
		fc.curBytes += info.Size()
		heap.Push(&fc.fih, info)
		return nil
	})
}

func (fc *FsCache) gcRunner() {
	for {
		select {
		case <-fc.gcStopCh:
			return
		case <-time.After(fc.gcInterval):
			fc.gc()
		}
	}
}

func (fc *FsCache) gc() {
	locked, err := fc.gcFlock.TryLock()
	if err != nil || !locked {
		return
	}
	defer fc.gcFlock.Unlock()

	fc.fihMu.Lock()
	defer fc.fihMu.Unlock()

	if fc.curBytes <= fc.maxBytes {
		return
	}

	var (
		needGcBytes = fc.curBytes - fc.maxBytes
		bytesSoFar  int64
		keysToGc    []string
	)

	for bytesSoFar < needGcBytes {
		fi := heap.Pop(&fc.fih).(os.FileInfo)
		bytesSoFar += fi.Size()
		keysToGc = append(keysToGc, fi.Name())
	}

	for _, k := range keysToGc {
		if err := os.Remove(fc.filepath(k)); err != nil {
			return
		}
	}
	fc.curBytes -= bytesSoFar
}

type Option func(fc *FsCache)

func WithCacheDir(cacheDir string) Option {
	return func(fc *FsCache) {
		fc.cacheDir = cacheDir
	}
}

func WithMaxBytes(bytes int64) Option {
	return func(fc *FsCache) {
		fc.maxBytes = bytes
	}
}

func WithGcInterval(interval time.Duration) Option {
	return func(fc *FsCache) {
		fc.gcInterval = interval
	}
}

func WithGcStopCh(stopCh <-chan struct{}) Option {
	return func(fc *FsCache) {
		fc.gcStopCh = stopCh
	}
}

func (f *FsCache) filepath(key string) string {
	return filepath.Join(f.cacheDir, "cache", key)
}

func (f *FsCache) tmppath(key string) string {
	return filepath.Join(f.cacheDir, "tmp", key)
}

// TODO update curBytes
func (f *FsCache) Set(key string, src io.Reader) error {
	return atomicWriteFile(f.filepath(key), f.tmppath(key), src, 0644)
}

func (f *FsCache) SetBytes(key string, src []byte) error {
	srcReader := bytes.NewReader(src)
	return atomicWriteFile(f.filepath(key), f.tmppath(key), srcReader, 0644)
}

func (f *FsCache) Get(key string, dst io.Writer) error {
	src, err := os.Open(f.filepath(key))
	if err != nil {
		if os.IsNotExist(err) {
			return ErrNotFound
		}
		return err
	}
	defer src.Close()
	if _, err := io.Copy(dst, src); err != nil {
		return err
	}
	return nil
}

func (f *FsCache) GetBytes(key string, dst []byte) ([]byte, error) {
	src, err := ioutil.ReadFile(f.filepath(key))
	if err != nil {
		if os.IsNotExist(err) {
			return dst, ErrNotFound
		}
		return dst, err
	}
	dst = append(dst, src...)
	return dst, nil
}

func (f *FsCache) Has(key string) bool {
	_, err := os.Stat(f.filepath(key))
	return err == nil
}

func (f *FsCache) Del(key string) {
	syscall.Unlink(f.filepath(key))
}
