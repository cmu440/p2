// DO NOT MODIFY!

package main

import (
	crand "crypto/rand"
	"flag"
	"log"
	"math"
	"math/big"
	"math/rand"

	"github.com/cmu440/tribbler/storageserver"
)

const defaultMasterPort = 9009

var (
	port           = flag.Int("port", defaultMasterPort, "port number to listen on")
	masterHostPort = flag.String("master", "", "master storage server host port (if non-empty then this storage server is a slave)")
	numNodes       = flag.Int("N", 1, "the number of nodes in the ring (including the master)")
	nodeID         = flag.Uint("id", 0, "a 32-bit unsigned node ID to use for consistent hashing")
)

func init() {
	log.SetFlags(log.Lshortfile | log.Lmicroseconds)
}

func main() {
	flag.Parse()
	if *masterHostPort == "" && *port == 0 {
		// If masterHostPort string is empty, then this storage server is the master.
		*port = defaultMasterPort
	}

	// If nodeID is 0, then assign a random 32-bit integer instead.
	randID := uint32(*nodeID)
	if randID == 0 {
		randint, _ := crand.Int(crand.Reader, big.NewInt(math.MaxInt64))
		rand.Seed(randint.Int64())
		randID = rand.Uint32()
	}

	// Create and start the StorageServer.
	_, err := storageserver.NewStorageServer(*masterHostPort, *numNodes, *port, randID)
	if err != nil {
		log.Fatalln("Failed to create storage server:", err)
	}

	// Run the storage server forever.
	select {}
}
