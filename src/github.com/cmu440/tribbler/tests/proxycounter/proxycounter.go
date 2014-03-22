// DO NOT MODIFY!

package proxycounter

import (
	"errors"
	"log"
	"net/rpc"
	"sync/atomic"

	"github.com/cmu440/tribbler/rpc/storagerpc"
	"github.com/cmu440/tribbler/storageserver"
)

type ProxyCounter interface {
	storageserver.StorageServer
	Reset()
	OverrideLeaseSeconds(leaseSeconds int)
	DisableLease()
	EnableLease()
	OverrideErr()
	OverrideStatus(status storagerpc.Status)
	OverrideOff()
	GetRpcCount() uint32
	GetByteCount() uint32
	GetLeaseRequestCount() uint32
	GetLeaseGrantedCount() uint32
}

type proxyCounter struct {
	srv                  *rpc.Client
	myhostport           string
	rpcCount             uint32
	byteCount            uint32
	leaseRequestCount    uint32
	leaseGrantedCount    uint32
	override             bool
	overrideErr          error
	overrideStatus       storagerpc.Status
	disableLease         bool
	overrideLeaseSeconds int
}

func init() {
	log.SetFlags(log.Lshortfile | log.Lmicroseconds)
}

func NewProxyCounter(serverHostPort, myHostPort string) (ProxyCounter, error) {
	pc := new(proxyCounter)
	pc.myhostport = myHostPort
	// Create RPC connection to storage server.
	srv, err := rpc.DialHTTP("tcp", serverHostPort)
	if err != nil {
		return nil, err
	}
	pc.srv = srv
	return pc, nil
}

func (pc *proxyCounter) Reset() {
	pc.rpcCount = 0
	pc.byteCount = 0
	pc.leaseRequestCount = 0
	pc.leaseGrantedCount = 0
}

func (pc *proxyCounter) OverrideLeaseSeconds(leaseSeconds int) {
	pc.overrideLeaseSeconds = leaseSeconds
}

func (pc *proxyCounter) DisableLease() {
	pc.disableLease = true
}

func (pc *proxyCounter) EnableLease() {
	pc.disableLease = false
}

func (pc *proxyCounter) OverrideErr() {
	pc.overrideErr = errors.New("error")
	pc.override = true
}

func (pc *proxyCounter) OverrideStatus(status storagerpc.Status) {
	pc.overrideStatus = status
	pc.override = true
}

func (pc *proxyCounter) OverrideOff() {
	pc.override = false
	pc.overrideErr = nil
	pc.overrideStatus = storagerpc.OK
}

func (pc *proxyCounter) GetRpcCount() uint32 {
	return pc.rpcCount
}

func (pc *proxyCounter) GetByteCount() uint32 {
	return pc.byteCount
}

func (pc *proxyCounter) GetLeaseRequestCount() uint32 {
	return pc.leaseRequestCount
}

func (pc *proxyCounter) GetLeaseGrantedCount() uint32 {
	return pc.leaseGrantedCount
}

// RPC methods.

func (pc *proxyCounter) RegisterServer(args *storagerpc.RegisterArgs, reply *storagerpc.RegisterReply) error {
	return nil
}

func (pc *proxyCounter) GetServers(args *storagerpc.GetServersArgs, reply *storagerpc.GetServersReply) error {
	err := pc.srv.Call("StorageServer.GetServers", args, reply)
	// Modify reply so node point to myself
	if len(reply.Servers) > 1 {
		panic("ProxyCounter only works with 1 storage node")
	} else if len(reply.Servers) == 1 {
		reply.Servers[0].HostPort = pc.myhostport
	}
	return err
}

func (pc *proxyCounter) Get(args *storagerpc.GetArgs, reply *storagerpc.GetReply) error {
	if pc.override {
		reply.Status = pc.overrideStatus
		return pc.overrideErr
	}
	byteCount := len(args.Key)
	if args.WantLease {
		atomic.AddUint32(&pc.leaseRequestCount, 1)
	}
	if pc.disableLease {
		args.WantLease = false
	}
	err := pc.srv.Call("StorageServer.Get", args, reply)
	byteCount += len(reply.Value)
	if reply.Lease.Granted {
		if pc.overrideLeaseSeconds > 0 {
			reply.Lease.ValidSeconds = pc.overrideLeaseSeconds
		}
		atomic.AddUint32(&pc.leaseGrantedCount, 1)
	}
	atomic.AddUint32(&pc.rpcCount, 1)
	atomic.AddUint32(&pc.byteCount, uint32(byteCount))
	return err
}

func (pc *proxyCounter) GetList(args *storagerpc.GetArgs, reply *storagerpc.GetListReply) error {
	if pc.override {
		reply.Status = pc.overrideStatus
		return pc.overrideErr
	}
	byteCount := len(args.Key)
	if args.WantLease {
		atomic.AddUint32(&pc.leaseRequestCount, 1)
	}
	if pc.disableLease {
		args.WantLease = false
	}
	err := pc.srv.Call("StorageServer.GetList", args, reply)
	for _, s := range reply.Value {
		byteCount += len(s)
	}
	if reply.Lease.Granted {
		if pc.overrideLeaseSeconds > 0 {
			reply.Lease.ValidSeconds = pc.overrideLeaseSeconds
		}
		atomic.AddUint32(&pc.leaseGrantedCount, 1)
	}
	atomic.AddUint32(&pc.rpcCount, 1)
	atomic.AddUint32(&pc.byteCount, uint32(byteCount))
	return err
}

func (pc *proxyCounter) Put(args *storagerpc.PutArgs, reply *storagerpc.PutReply) error {
	if pc.override {
		reply.Status = pc.overrideStatus
		return pc.overrideErr
	}
	byteCount := len(args.Key) + len(args.Value)
	err := pc.srv.Call("StorageServer.Put", args, reply)
	atomic.AddUint32(&pc.rpcCount, 1)
	atomic.AddUint32(&pc.byteCount, uint32(byteCount))
	return err
}

func (pc *proxyCounter) AppendToList(args *storagerpc.PutArgs, reply *storagerpc.PutReply) error {
	if pc.override {
		reply.Status = pc.overrideStatus
		return pc.overrideErr
	}
	byteCount := len(args.Key) + len(args.Value)
	err := pc.srv.Call("StorageServer.AppendToList", args, reply)
	atomic.AddUint32(&pc.rpcCount, 1)
	atomic.AddUint32(&pc.byteCount, uint32(byteCount))
	return err
}

func (pc *proxyCounter) RemoveFromList(args *storagerpc.PutArgs, reply *storagerpc.PutReply) error {
	if pc.override {
		reply.Status = pc.overrideStatus
		return pc.overrideErr
	}
	byteCount := len(args.Key) + len(args.Value)
	err := pc.srv.Call("StorageServer.RemoveFromList", args, reply)
	atomic.AddUint32(&pc.rpcCount, 1)
	atomic.AddUint32(&pc.byteCount, uint32(byteCount))
	return err
}
