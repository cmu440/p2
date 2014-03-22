// This file contains constants and arguments used to perform RPCs between
// a TribServer's local Libstore and the storage servers. DO NOT MODIFY!

package storagerpc

// Status represents the status of a RPC's reply.
type Status int

const (
	OK           Status = iota + 1 // The RPC was a success.
	KeyNotFound                    // The specified key does not exist.
	ItemNotFound                   // The specified item does not exist.
	WrongServer                    // The specified key does not fall in the server's hash range.
	ItemExists                     // The item already exists in the list.
	NotReady                       // The storage servers are still getting ready.
)

// Lease constants.
const (
	QueryCacheSeconds = 10 // Time period used for tracking queries/determining whether to request leases.
	QueryCacheThresh  = 3  // If QueryCacheThresh queries in last QueryCacheSeconds, then request a lease.
	LeaseSeconds      = 10 // Number of seconds a lease should remain valid.
	LeaseGuardSeconds = 2  // Additional seconds a server should wait before invalidating a lease.
)

// Lease stores information about a lease sent from the storage servers.
type Lease struct {
	Granted      bool
	ValidSeconds int
}

type Node struct {
	HostPort string // The host:port address of the storage server node.
	NodeID   uint32 // The ID identifying this storage server node.
}

type RegisterArgs struct {
	ServerInfo Node
}

type RegisterReply struct {
	Status  Status
	Servers []Node
}

type GetServersArgs struct {
	// Intentionally left empty.
}

type GetServersReply struct {
	Status  Status
	Servers []Node
}

type GetArgs struct {
	Key       string
	WantLease bool
	HostPort  string // The Libstore's callback host:port.
}

type GetReply struct {
	Status Status
	Value  string
	Lease  Lease
}

type GetListReply struct {
	Status Status
	Value  []string
	Lease  Lease
}

type PutArgs struct {
	Key   string
	Value string
}

type PutReply struct {
	Status Status
}

type RevokeLeaseArgs struct {
	Key string
}

type RevokeLeaseReply struct {
	Status Status
}
