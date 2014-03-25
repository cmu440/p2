// This file provides a type-safe wrapper that should be used to register the
// storage server to receive RPCs from a TribServer's libstore. DO NOT MODIFY!

package storagerpc

// STAFF USE ONLY! Students should not use this interface in their code.
type RemoteStorageServer interface {
	RegisterServer(*RegisterArgs, *RegisterReply) error
	GetServers(*GetServersArgs, *GetServersReply) error
	Get(*GetArgs, *GetReply) error
	GetList(*GetArgs, *GetListReply) error
	Put(*PutArgs, *PutReply) error
	AppendToList(*PutArgs, *PutReply) error
	RemoveFromList(*PutArgs, *PutReply) error
}

type StorageServer struct {
	// Embed all methods into the struct. See the Effective Go section about
	// embedding for more details: golang.org/doc/effective_go.html#embedding
	RemoteStorageServer
}

// Wrap wraps s in a type-safe wrapper struct to ensure that only the desired
// StorageServer methods are exported to receive RPCs.
func Wrap(s RemoteStorageServer) RemoteStorageServer {
	return &StorageServer{s}
}
