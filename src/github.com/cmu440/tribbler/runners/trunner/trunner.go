// DO NOT MODIFY!

package main

import (
	"flag"
	"log"
	"net"
	"strconv"

	"github.com/cmu440/tribbler/tribserver"
)

var port = flag.Int("port", 9010, "port number to listen on")

func init() {
	log.SetFlags(log.Lshortfile | log.Lmicroseconds)
}

func main() {
	flag.Parse()
	if flag.NArg() < 1 {
		log.Fatalln("Usage: trunner <master storage server host:port>")
	}

	// Create and start the TribServer.
	hostPort := net.JoinHostPort("localhost", strconv.Itoa(*port))
	_, err := tribserver.NewTribServer(flag.Arg(0), hostPort)
	if err != nil {
		log.Fatalln("Server could not be created:", err)
	}

	// Run the Tribbler server forever.
	select {}
}
