// The static cache used to store the line number offsets of
// each line in the file.  The cache is intentionally "variably sparse"
// as a time/space tradeoff.  If a given line number is not found
// in the cache, the line number with the highest value less than
// the target line number is read, and from there enough lines are
// read to get to the desired line.
//
// We are given a requested buffer length and determine the total number
// of lines in the file.  The number of lines we actually store is
// generally less than all, but we attempt to maintain uniform spacing
// in the gaps.
//
// We can fine tune this better with a more sophisticated skipping method,
// but for the sake of simplicity, we'll determine the number N such that
// (1/N) of the lines in the file will fit in the cache
//
// Example:
//     requested buffer length = 1000
//     actual lines in file = 7342
//     Then 1 out of every 8 lines will fit in the cache.
// The idea is to attain a good spread of entries such that
// we can tend to minimize the number of lines read on average.
// Note the cache may not be fully used, but this is where a
// more sophisticated method would increase the precision.
// Note we assume the file contents never change, and that it is
// newline-terminated text.
//
// One reason we do not ever change the cache is that the problem
// statement does not hint at any kind of locality or repitition
// of references.  So if we cached new uncached line numbers, we'd
// end up destroying the uniform spacing.
package main

import (
	"bufio"
	"bytes"
	"fmt"
	"io"
	"log"
	"os"
)

// IndexCache is an interface that defines the methods for any line
// server cache.  Thus we are not licked to this particular implmentation.
// Normally this interface declaration would go in a different file/package.
type IndexCache interface {
	Lookup(lineno int64) (string, error)
}

// Per-line data.
type lineInfo struct {
	lineno int64
	offset int64
}

// The static cache object.  Note for the sake of concurrency, we
// store the filename, not the os.File object.
type lineOffsetCache struct {
	filename string
	cache    []lineInfo
	totLines int64
}

var (
	// Ensure type conforms to interface.
	_ IndexCache = (*lineOffsetCache)(nil)
)

// NewLineOffsetCache creates a new cache object given a file name and
// target cache size.  Returns an error if something went wrong.  The
// design-specifics and algorith are documented in the header comment.
func newLineOffsetCache(filename string, cacheSize int) (
	*lineOffsetCache, error) {
	f, err := os.OpenFile(filename, os.O_RDWR, 0)
	if err != nil {
		log.Printf("error opening target file: '%v'\n", err)
		return nil, err
	}
	defer func() {
		if err := f.Close(); err != nil {
			log.Printf("warning: unable to close search file: %v\n", err)
		}
	}()

	// We need the line count to determine the spacing.
	cnt, err := getLineCount(f)
	if err != nil {
		log.Printf("error getting line count: '%v'\n", err)
		return nil, err
	}

	// Rewind to the beginning to build the actual cache.
	_, err = f.Seek(0, 0)
	if err != nil {
		log.Printf("seek error: '%v'\n", err)
		return nil, err
	}
	cache, err := buildCache(f, cnt, cacheSize)
	if err != nil {
		log.Printf("read error building cache: '%v'\n", err)
		return nil, err
	}
	return &lineOffsetCache{cache: cache, filename: filename, totLines: cnt},
		nil
}

// Lookup a string given the line number.  Note we used a 0-based
// line cache, but the user interface is 1-based, so we adjust.
// Note the trailing newline is left intact.
func (loc *lineOffsetCache) Lookup(lineno int64) (string, error) {
	lineno--
	if lineno < 0 || lineno >= loc.totLines {
		return "", fmt.Errorf(
			"invalid requested line number '%d': %d lines in file",
			lineno+1, loc.totLines)
	}

	li := findLineInfo(lineno, loc.cache)
	if li != nil && li.lineno > lineno {
		// Should not happen.
		return "",
			fmt.Errorf("unexpected search error for line number '%d'", lineno)
	}

	f, err := os.OpenFile(loc.filename, os.O_RDWR, 0)
	if err != nil {
		log.Printf("error opening target file: '%v'\n", err)
		return "", err
	}
	defer func() {
		if err := f.Close(); err != nil {
			log.Printf("warning: unable to close search file: %v\n", err)
		}
	}()

	var begin int64
	var seekTo int64
	var res string
	if li == nil {
		// No such animal in the cache, so our line number is below the min.
		begin = 0
		seekTo = 0
	} else {
		//value found is less than or equal to the requested line number.
		begin = li.lineno
		seekTo = li.offset
	}

	_, err = f.Seek(seekTo, 0)
	if err != nil {
		return "", err
	}
	r := bufio.NewReader(f)
	for ndx := begin; ndx <= lineno; ndx++ {
		b, err := r.ReadBytes('\n')
		if err != nil {
			return "", err
		}
		if ndx == lineno {
			res = string(b)
		}
	}
	return res, nil
}

// Finds the line info for the entry that has the largest line number
// less than or equal to the desired line number. using a binary search.
// Note: this version of the code is the result of manually unrolling
// tail recursion to avoid a recursive search, so it is slightly
// less visually appealing.
func findLineInfo(linenum int64, li []lineInfo) *lineInfo {
	s := li
	for {
		if len(s) == 0 {
			return nil
		} else if len(s) == 1 {
			if s[0].lineno < linenum {
				return &s[0]
			}
			return nil
		}

		mid := len(s) / 2
		mv := s[mid].lineno
		if linenum == mv {
			return &s[mid]
		} else if linenum < mv {
			s = s[:mid]
		} else {
			s = s[mid:]
		}
	}
}

// Count the number of lines in the file in an optimized manner.
func getLineCount(f *os.File) (int64, error) {
	r := bufio.NewReader(f)

	// Modified slightly from: https://stackoverflow.com/questions/24562942/
	// golang-how-do-i-determine-the-number-of-lines-in-a-file-efficiently
	buf := make([]byte, 32*1024)
	count := int64(0)
	lineSep := []byte{'\n'}

	for {
		c, err := r.Read(buf)
		count += int64(bytes.Count(buf[:c], lineSep))

		switch {
		case err == io.EOF:
			return count, nil

		case err != nil:
			return count, err
		}
	}
}

// Build the cache given the total number of lines and target
// cache size.
func buildCache(f *os.File, lineCnt int64,
	cacheLen int) ([]lineInfo, error) {
	// Given the line count and requested buffer length, determine
	// how many lines to actually store, attempting to maintain
	// uniform spacing.
	if lineCnt < int64(cacheLen) {
		// Cache is larger than the number of lines in the file, so
		// index every line.
		return processLines(f, make([]lineInfo, lineCnt), lineCnt, 1)
	}

	// Cache is smaller than line count, so include equally spaced
	// (by line number) indices.
	li := make([]lineInfo, cacheLen)
	skipFactor := lineCnt / int64(cacheLen)
	if lineCnt%int64(cacheLen) != 0 {
		skipFactor++
	}
	return processLines(f, li, lineCnt, skipFactor)
}

// Populate the cache with every one of every "skip_factor"
// lines read.
func processLines(f *os.File, li []lineInfo, numLines, skipFactor int64) (
	[]lineInfo, error) {

	// One goal in this loop is to avoid unnecessary slice creation,
	// so we'll read single buffered bytes instead of depending on
	// ReadBytes(delim byte).
	saveOff := int64(0)
	offset := int64(0)
	nextSlot := int64(0)
	r := bufio.NewReader(f)
	for line := int64(0); line < numLines; {
		b, err := r.ReadByte()
		if err != nil {
			if err == io.EOF {
				break
			} else {
				return nil, err
			}
		}
		if b == '\n' {
			if line%skipFactor == 0 {
				// Add this line to the cache.
				li[nextSlot] = lineInfo{lineno: line, offset: saveOff}
				nextSlot++
			}
			saveOff = offset + 1
			line++
		}
		offset++
	}
	return li[:nextSlot], nil
}
