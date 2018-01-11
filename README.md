# lserver
Sample Go code

This is my implmentation of the well-know problem of efficently retrieving lines in a file.

You are asked to create an efficient and scalable server to return any line of a specified file (the line numbers are 1-based).  The user specifies the file and an upper bound on the number of cached entries.

My solution builds a cache that stores the byte offsets for every nth line of the file, where n is a rough best fit for the size of the cache.  Note this is a static cache, as there is nothing in the problem statement that implies any "hot spots" or locality of requests, so a static cache seems like a reasonable choice.

Commands:

* GET <(1-based) line number>
* QUIT
* SHUTDOWN

Sample client usage:
* $ echo "GET 7777" | nc localhost 8080
* $ echo "SHUTDOWN" | nc localhost 8080
* $ nc localhost 8080 << HERE
> GET 3456
> GET 1234
> QUIT
> HERE

On the server side, the only mandatory argument is the pathname of a file to index.
Options are: -server_addr defaults to "localhost:8080"
             -cache_size upper bound on number of cached entries, default is one million (1024*1024) entries. 
