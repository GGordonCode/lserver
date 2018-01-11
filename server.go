package main

import (
	"bufio"
	"fmt"
	"log"
	"net"
	"strconv"
	"strings"
	"sync"
)

const (
	getCmd      = "GET"
	quitCmd     = "QUIT"
	shutdownCmd = "SHUTDOWN"
)

type server struct {
	addr       string
	listener   net.Listener
	cache      IndexCache
	mu         sync.Mutex
	isShutdown bool
}

func newServer(addr string, cache IndexCache) *server {
	return &server{addr: addr, cache: cache}
}

func (s *server) processRequests() (err error) {
	defer func() {
		if s.listener != nil {
			s.listener.Close()
		}
	}()

	s.listener, err = net.Listen("tcp", s.addr)
	if err != nil {
		return err
	}

	host, port, err := net.SplitHostPort(s.addr)
	if err != nil {
		log.Printf("Cannot parse server address\n", err)
		return err
	}
	if port == "0" {
		// System chooses port number.
		log.Printf("Server listening for connections on %s:%d.\n", host,
			s.listener.Addr().(*net.TCPAddr).Port)
	} else {
		log.Printf("Server listening for connections on %s.\n", s.addr)
	}

	for {
		conn, err := s.listener.Accept()
		if err != nil {
			if !s.isShutdown {
				log.Printf("Accept() error: %v\n", err)
				return err
			}
			return nil
		}
		go s.handleConnection(conn)
	}
	return nil
}

func (s *server) handleConnection(conn net.Conn) {
	defer conn.Close()

	scanner := bufio.NewScanner(conn)
	for scanner.Scan() {
		err := scanner.Err()
		if err != nil {
			log.Printf("connection read error: %v\n", err)
			break
		}
		line := strings.TrimSpace(scanner.Text())
		parts := strings.Split(line, " ")
		if (len(parts) == 1) && (parts[0] == quitCmd) {
			break
		} else if (len(parts) == 1) && (parts[0] == shutdownCmd) {
			s.shutdown()
			break
		} else if (len(parts) == 2) && (parts[0] == getCmd) {
			val, err := strconv.ParseInt(parts[1], 10, 64)
			if err != nil {
				conn.Write([]byte(fmt.Sprintf("Error: invalid line number '%s'\n",
					parts[1])))
			}
			str, err := s.cache.Lookup(val)
			if err != nil {
				conn.Write([]byte(fmt.Sprintf("Error: lookup failed for '%d': %v\n",
					val, err)))
			} else {
				conn.Write([]byte(str))
			}
		} else {
			conn.Write([]byte(fmt.Sprintf("Error: invalid request: '%s'\n", line)))
		}
	}
}

func (s *server) getPort() int {
	if s.listener != nil {
		return s.listener.Addr().(*net.TCPAddr).Port
	}
	return -1
}

// On shutdown, close the listener.  With a bit more work, we could
// track each active client, but for the purposes of this demo, we
// don't deal with that.
func (s *server) shutdown() {
	if s.isShutdown {
		return
	}
	s.mu.Lock()
	fmt.Printf("Shutting down server...\n")
	s.isShutdown = true
	if s.listener != nil {
		s.listener.Close()
	}
	s.mu.Unlock()
}
