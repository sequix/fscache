package fscache

import (
	"bytes"
	"io/ioutil"
	"math/rand"
	"os"
	"testing"
	"time"
)

func randBytes(n int) []byte {
	rst := make([]byte, n)
	_, err := rand.Read(rst)
	if err != nil {
		panic(err)
	}
	return rst
}

func TestMain(m *testing.M) {
	rand.Seed(time.Now().UnixNano())
	m.Run()
}

func newCache() (cache *Cache, cancel func()) {
	cacheDir, err := ioutil.TempDir("", "fscache")
	if err != nil {
		panic(err)
	}
	gcStopCh := make(chan struct{})

	cacheI, err := New(
		WithCacheDir(cacheDir),
		WithMaxBytes(3*1024),
		WithGcInterval(2*time.Second),
		WithGcStopCh(gcStopCh),
	)
	if err != nil {
		panic(err)
	}
	cache = cacheI.(*Cache)
	cancel = func() {
		close(gcStopCh)
		if err := os.RemoveAll(cacheDir); err != nil {
			panic(err)
		}
	}
	return
}

func TestSetHasGet(t *testing.T) {
	cache, cancel := newCache()
	defer cancel()

	key := "key"
	val := randBytes(1024)
	if err := cache.Set(key, val); err != nil {
		panic(err)
	}

	valFromCache, err := cache.Get(key, nil)
	if err != nil {
		panic(err)
	}

	if !bytes.Equal(val, valFromCache) {
		t.Errorf("valFromCache not equals to val")
	}

	val = randBytes(1024)
	if err := cache.Set(key, val); err != nil {
		panic(err)
	}

	valFromCache, err = cache.Get(key, nil)
	if err != nil {
		panic(err)
	}

	if !bytes.Equal(val, valFromCache) {
		t.Errorf("valFromCache not equals to val")
	}

	_, err = cache.Get("notFound", nil)
	if err != ErrNotFound {
		t.Errorf("expected not found error")
	}

	if !cache.Has(key) {
		t.Errorf("expected Has() returning true")
	}

	if cache.Has("notFound") {
		t.Errorf("expected Has() returning false")
	}
}

func TestGc(t *testing.T) {
	cache, cancel := newCache()
	defer cancel()

	cm := map[string][]byte{
		"key1": randBytes(3 * 1024),
		"key2": randBytes(1024),
		"key3": randBytes(1024),
	}

	for key, val := range cm {
		if err := cache.Set(key, val); err != nil {
			panic(err)
		}
	}

	// to update atime of key1
	if _, err := cache.Get("key1", nil); err != nil {
		panic(err)
	}

	time.Sleep(3 * time.Second)

	if !cache.Has("key1") {
		t.Errorf("expected Has() returning true for key1")
	}

	if cache.Has("key2") {
		t.Errorf("expected Has() returning false for key2")
	}

	if cache.Has("key3") {
		t.Errorf("expected Has() returning false for key3")
	}
}
