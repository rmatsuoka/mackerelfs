package main

import (
	"log"
	"net"
	"os"

	"github.com/mackerelio/mackerel-client-go"
	"github.com/rmatsuoka/mackerelfs"
	"github.com/rmatsuoka/ya9p"
)

func main() {
	listener, err := net.Listen("tcp", "localhost:8000")
	if err != nil {
		log.Fatal(err)
	}
	client, err := mackerel.NewClientWithOptions(
		os.Getenv("MACKEREL_APIKEY"),
		"https://api.mackerelio.com/",
		true,
	)
	if err != nil {
		log.Fatal(err)
	}

	for {
		conn, err := listener.Accept()
		if err != nil {
			log.Print(err)
		}
		go ya9p.Serve(conn, ya9p.FS(mackerelfs.HostFS(client)))
	}
}
