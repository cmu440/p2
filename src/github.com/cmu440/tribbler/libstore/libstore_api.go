// DO NOT MODIFY!

package libstore

import (
	"hash/fnv"

	"github.com/cmu440/tribbler/rpc/storagerpc"
)

// LeaseMode is a debugging flag that determines how the Libstore should
// request/handle leases.
type LeaseMode int

const (
	Never  LeaseMode = iota // Never request leases.
	Normal                  // Behave as normal.
	Always                  // Always request leases.
)

// Libstore defines the set of methods that a TribServer can call on its
// local cache.
type Libstore interface {
	Get(key string) (string, error)
	Put(key, value string) error
	GetList(key string) ([]string, error)
	AppendToList(key, newItem string) error
	RemoveFromList(key, removeItem string) error
}

// LeaseCallbacks defines the set of methods that a StorageServer can call
// on a TribServer's local cache.
type LeaseCallbacks interface {

	// RevokeLease is a callback RPC method that is invoked by storage
	// servers when a lease is revoked. It should reply with status OK
	// if the key was successfully revoked, or with status KeyNotFound
	// if the key did not exist in the cache.
	RevokeLease(*storagerpc.RevokeLeaseArgs, *storagerpc.RevokeLeaseReply) error
}

// StoreHash hashes a string key and returns a 32-bit integer. This function
// is provided here so that all implementations use the same hashing mechanism
// (both the Libstore and StorageServer should use this function to hash keys).
func StoreHash(key string) uint32 {
	hasher := fnv.New32()
	hasher.Write([]byte(key))
	return hasher.Sum32()
}
