package fscache

import (
	"container/heap"
	"errors"
	"io/ioutil"
	"log"
	"math"
	"os"
	"path/filepath"
	"time"
)

// Interface provides a set of general cache functions.
type Interface interface {
	// Set sets the value of key as src.
	// Setting the same key multiple times, the last set call takes effect.
	Set(key string, src []byte) error
	// Get gets the value of key to dst, and returns dst no matter whether or not there is an error.
	Get(key string, dst []byte) ([]byte, error)
	// Has tells you if a key has been set or not.
	Has(key string) bool
}

var (
	// ErrNotFound will be returned when getting a key that not setting before.
	ErrNotFound = errors.New("not found")
)

// Cache is a LRU filesystem cache based on atime.
type Cache struct {
	cacheDir   string
	maxBytes   int64
	gcInterval time.Duration
	logger     Logger
	fih        fileInfoHeap
	gcStopCh   <-chan struct{}
}

func (f *Cache) filedir() string            { return filepath.Join(f.cacheDir, "cache") }
func (f *Cache) tmpdir() string             { return filepath.Join(f.cacheDir, "tmp") }
func (f *Cache) filepath(key string) string { return filepath.Join(f.filedir(), key) }
func (f *Cache) tmppath(key string) string  { return filepath.Join(f.tmpdir(), key) }

// Option can be passed to New() to tailor your needs.
type Option func(fc *Cache)

// WithCacheDir specifies where the cache holds.
func WithCacheDir(cacheDir string) Option { return func(fc *Cache) { fc.cacheDir = cacheDir } }

// WithMaxBytes specifies how many space the cache could take up.
func WithMaxBytes(bytes int64) Option { return func(fc *Cache) { fc.maxBytes = bytes } }

// WithGcStopCh receives a channel, when the channel close, gc will stop.
// By default, gc will not stop until the process exits.
func WithGcStopCh(stopCh <-chan struct{}) Option { return func(fc *Cache) { fc.gcStopCh = stopCh } }

// WithGcInterval specifies how often the GC performs.
func WithGcInterval(interval time.Duration) Option {
	return func(fc *Cache) { fc.gcInterval = interval }
}

// Logger used by this package.
type Logger interface {
	Errorf(fmt string, args ...interface{})
}

type logger struct {
	log.Logger
}

func (l *logger) Errorf(fmt string, args ...interface{}) { log.Printf(fmt, args...) }

// New creates a LRU filesystem cache based on atime, and starts the GC goroutine.
func New(opts ...Option) (Interface, error) {
	fc := &Cache{
		cacheDir:   os.TempDir(),
		maxBytes:   math.MaxInt64,
		gcInterval: 5 * time.Minute,
		logger:     &logger{},
		gcStopCh:   make(chan struct{}),
	}
	for _, opt := range opts {
		opt(fc)
	}
	if err := os.MkdirAll(fc.filedir(), 0775); err != nil {
		return nil, err
	}
	if err := os.MkdirAll(fc.tmpdir(), 0775); err != nil {
		return nil, err
	}
	if fc.maxBytes > 0 {
		go fc.gcRunner()
	}
	return fc, nil
}

func (f *Cache) gcRunner() {
	ticker := time.NewTicker(f.gcInterval)
	defer ticker.Stop()
	for {
		select {
		case <-f.gcStopCh:
			return
		case <-ticker.C:
			f.gc()
		}
	}
}

func (f *Cache) gc() {
	curBytes := int64(0)
	f.fih = nil

	err := filepath.Walk(f.filedir(), func(path string, info os.FileInfo, err error) error {
		if err != nil {
			return err
		}
		if info.IsDir() {
			return nil
		}
		curBytes += info.Size()
		heap.Push(&f.fih, info)
		return nil
	})
	if err != nil {
		f.logger.Errorf("gc walk dir %s : %s", f.filedir(), err)
		return
	}

	if curBytes <= f.maxBytes {
		return
	}

	var (
		needGcBytes = curBytes - f.maxBytes
		bytesSoFar  int64
		keysToGc    []string
	)

	for bytesSoFar < needGcBytes {
		fi := heap.Pop(&f.fih).(os.FileInfo)
		bytesSoFar += fi.Size()
		keysToGc = append(keysToGc, fi.Name())
	}

	for _, k := range keysToGc {
		fp := f.filepath(k)
		if err := os.Remove(fp); err != nil {
			f.logger.Errorf("gc %s : %s", fp, err)
			return
		}
	}
}

// Set implements Interface.Set().
func (f *Cache) Set(key string, src []byte) error {
	return atomicWriteFile(f.filepath(key), f.tmppath(key), src, 0644)
}

// Get implements Interface.Get().
func (f *Cache) Get(key string, dst []byte) ([]byte, error) {
	fp := f.filepath(key)
	src, err := ioutil.ReadFile(fp)
	if err != nil {
		if os.IsNotExist(err) {
			return dst, ErrNotFound
		}
		return dst, err
	}
	fi, err := os.Stat(fp)
	if err != nil {
		return dst, err
	}
	if err := os.Chtimes(fp, time.Now(), fi.ModTime()); err != nil {
		return dst, err
	}
	dst = append(dst, src...)
	return dst, nil
}

// Has implements Interface.Has().
func (f *Cache) Has(key string) bool {
	_, err := os.Stat(f.filepath(key))
	return err == nil
}
