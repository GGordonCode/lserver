package main

import (
	"bufio"
	"fmt"
	"io/ioutil"
	"os"
	"path"
	"testing"
)

// TODO: add more tests!

func TestCache(t *testing.T) {
	tmpdir, err := ioutil.TempDir("", "cache_test")
	if err != nil {
		t.Fatalf("temp dir: %v", err)
	}
	defer os.RemoveAll(tmpdir) // clean up afterwards

	// Create a temp dir to store the test cache file in.
	// Remove the directory at the end of the test.
	cp := path.Join(tmpdir, "testfile.txt")
	doTestCacheFile(t, cp, 3000, 32456, 0, 10000, 55555)
	doTestCacheFile(t, cp, 10, 21, 13)
	doTestCacheFile(t, cp, 40, 199, 0, 13, 198, 199, 200, 201)
	doTestCacheFile(t, cp, 40, 200, 13, 198, 199, 200, 201)
	doTestCacheFile(t, cp, 40, 201, 13)
	doTestCacheFile(t, cp, 40, 202, 113)
	doTestCacheFile(t, cp, 40, 259, 213)
	doTestCacheFile(t, cp, 40, 399, 313)
	doTestCacheFile(t, cp, 2500, 2500, 500)

	// Big data tests
	doTestCacheFile(t, cp, 1024*1024, 10*1024*1024, 7*1024*1024+539)
	doTestCacheFile(t, cp, 1024*1024, 10*1024*1024+1, 3*1024*1024+107)
	doTestCacheFile(t, cp, 1024*1024, 5*1024*1024+999, 5*1024*1024+999, 1,
		65536, 1024*1024+389)
}

func doTestCacheFile(t *testing.T, filename string, cacheSize int,
	lineCnt int64, targetLine ...int64) {
	f, err := os.Create(filename)
	if err != nil {
		t.Fatalf("create temp file: %v", err)
	}
	closed := false
	defer os.Remove(filename)
	defer func() {
		if !closed {
			f.Close()
		}
	}()

	w := bufio.NewWriter(f)
	for i := int64(0); i < lineCnt; i++ {
		_, err := w.Write([]byte(fmt.Sprintf("Here is line %d.\n", i+1)))
		if err != nil {
			t.Fatalf("write temp file: %v", err)
		}
	}
	w.Flush()
	if err = f.Close(); err != nil {
		t.Fatalf("close temp file: %v", err)
	}
	closed = true

	c, err := newLineOffsetCache(filename, cacheSize)
	if err != nil {
		t.Fatalf("create cache: %v", err)
	}

	if cacheSize != len(c.cache) {
		t.Fatalf("expected cache size %d, but got %d\n", cacheSize,
			len(c.cache))
	}

	for _, v := range targetLine {
		l, err := c.Lookup(v)
		if v <= 0 || v > lineCnt {
			if err == nil {
				t.Fatalf("no error for lookup invalid line: %d\n", v)
			}
			continue
		} else if err != nil {
			t.Fatalf("lookup line %d: %v", v, err)
		}

		if l != fmt.Sprintf("Here is line %d.\n", v) {
			t.Fatalf("lookup line unexpected result for line %d: '%s'\n",
				v, l)
		}
	}
}
