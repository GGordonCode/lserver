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
	doTestCacheFile(t, cp, 3000, 32456, 10000)
	doTestCacheFile(t, cp, 10, 21, 13)
	doTestCacheFile(t, cp, 2500, 2500, 500)

	// Big data tests
	doTestCacheFile(t, cp, 1024*1024, 10*1024*1024, 7*1024*1024+539)
	doTestCacheFile(t, cp, 1024*1024, 10*1024*1024+1, 3*1024*1024+107)
	doTestCacheFile(t, cp, 1024*1024, 5*1024*1024+999, 5*1024*1024+999)
}

func doTestCacheFile(t *testing.T, filename string, cacheSize int,
	lineCnt, targetLine int64) {
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

	var expectedCacheSize int
	if lineCnt > int64(cacheSize) {
		if lineCnt%int64(cacheSize) == 0 {
			expectedCacheSize = cacheSize
		} else if lineCnt%int64(cacheSize) == 1 {
			expectedCacheSize = int(lineCnt / (lineCnt/int64(cacheSize) + 1))
		} else {
			expectedCacheSize = int(lineCnt/(lineCnt/int64(cacheSize)+1) + 1)
		}
	} else {
		expectedCacheSize = cacheSize
	}
	if expectedCacheSize != len(c.cache) {
		t.Fatalf("expected cache size %d, but got %d\n", expectedCacheSize,
			len(c.cache))
	}
	l, err := c.Lookup(targetLine)
	if err != nil {
		t.Fatalf("lookup line %d: %v", targetLine, err)
	}

	if l != fmt.Sprintf("Here is line %d.\n", targetLine) {
		t.Fatalf("lookup line unexpected result: '%s'\n", l)
	}

	l, err = c.Lookup(0)
	if err == nil {
		t.Fatalf("no error for lookup line 0\n")
	}
}
