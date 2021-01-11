package fscache

import (
	"bytes"
	"errors"
	"io"
	"io/ioutil"
	"os"
	"path/filepath"
	"syscall"
	"time"
)

// TODO: lru gc; doc; readme; test; init

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
	gcInterval time.Duration
	fih        fileInfoHeap
}

func New(opts ...Option) (Interface, error) {
	fc := &FsCache{}
	for _, opt := range opts {
		opt(fc)
	}
	return fc, nil
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

func (f *FsCache) filepath(key string) string {
	return filepath.Join(f.cacheDir, "cache", key)
}

func (f *FsCache) tmppath(key string) string {
	return filepath.Join(f.cacheDir, "tmp", key)
}

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