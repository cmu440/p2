// DO NOT MODIFY!

package main

import (
	"flag"
	"log"

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
	_, err := tribserver.NewTribServer(flag.Arg(0), *port)
	if err != nil {
		log.Fatalln("Server could not be created:", err)
	}

	// Run the Tribbler server forever.
	select {}
}
