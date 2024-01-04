package main

import (
	"log"
	"net"

	"github.com/rmatsuoka/mackerelfs"
	"github.com/rmatsuoka/ya9p"
)

func main() {
	listener, err := net.Listen("tcp", "localhost:8000")
	if err != nil {
		log.Fatal(err)
	}

	if err != nil {
		log.Fatal(err)
	}

	fsys := mackerelfs.FS()
	for {
		conn, err := listener.Accept()
		if err != nil {
			log.Print(err)
		}
		go ya9p.Serve(conn, ya9p.FS(fsys))
	}
}
