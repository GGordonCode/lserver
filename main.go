// The line-server program is a TCP server that returns the text at
// requested line numbers from a specified text file.  The user
// connects to the server and sends single-line commands to the server
// to get a line, quit the session (EOF also works), or shutdown.
//
// Commands:
// GET <(1-based) line number>
// QUIT
// SHUTDOWN
//
// Sample client usage:
// $ echo "GET 7777" | nc localhost 8080
// $ echo "SHUTDOWN" | nc localhost 8080
//
// nc localhost 8080 << HERE
// > GET 3456
// > GET 1234
// > QUIT
// > HERE
package main

import (
	"flag"
	"fmt"
	"log"
	"os"
	"os/signal"
	"syscall"
)

// The LineServer has a server implmentation, which in turn controls
// the cache.
type lineServer struct {
	server *server
}

// The user may configuer the IP address and cache size.  The algorithm
// used for the cache, and it's relation to the cache size is docuemnted
// in cache.go.
var (
	addr = flag.String("server_addr", "localhost:8080",
		"the address the server will listen for requests on")
	cacheSize = flag.Int("cache_size", 1024*1024,
		"recommended number of items to retain in the cache")
)

func main() {
	flag.Parse()
	if flag.NArg() < 1 {
		fmt.Fprintf(os.Stderr, "usage: %s <target file>\n", os.Args[0])
		os.Exit(1)
	}

	ls := &lineServer{}

	// Buuld the static cache first.
	cache, err := newLineOffsetCache(flag.Arg(0), *cacheSize)
	if err != nil {
		log.Printf("error creating cache: '%v'\n", err)
		os.Exit(1)
	}

	// Build the TCP server, passing it the cache.
	ls.server = newServer(*addr, cache)

	// Shutdown cleanup on termination signal (SIGINT and SIGTERM for now).
	go func() {
		ch := make(chan os.Signal)
		signal.Notify(ch, syscall.SIGINT, syscall.SIGTERM)
		log.Println(<-ch)

		// The server will clean up all necessary resources.
		ls.server.shutdown()
	}()

	// Loop, handling requests, until we are told to shutdown by a
	// user command, or a signal is received.
	var code int
	if err = ls.server.processRequests(); err != nil {
		log.Printf("error processing requests: '%v'\n", err)
		code = 1
	}
	os.Exit(code)
}
