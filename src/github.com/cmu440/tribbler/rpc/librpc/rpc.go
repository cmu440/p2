// This file provides a type-safe wrapper that should be used to register
// the libstore to receive RPCs from the storage server. DO NOT MODIFY!

package librpc

import "github.com/cmu440/tribbler/rpc/storagerpc"

type RemoteLeaseCallbacks interface {
	RevokeLease(*storagerpc.RevokeLeaseArgs, *storagerpc.RevokeLeaseReply) error
}

type LeaseCallbacks struct {
	// Embed all methods into the struct. See the Effective Go section about
	// embedding for more details: golang.org/doc/effective_go.html#embedding
	RemoteLeaseCallbacks
}

// Wrap wraps l in a type-safe wrapper struct to ensure that only the desired
// LeaseCallbacks methods are exported to receive RPCs.
func Wrap(l RemoteLeaseCallbacks) RemoteLeaseCallbacks {
	return &LeaseCallbacks{l}
}
