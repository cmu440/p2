package main

import (
	"flag"
	"fmt"
	"log"
	"net"
	"net/http"
	"net/rpc"
	"os"
	"regexp"
	"runtime"
	"strings"
	"time"

	"github.com/cmu440/tribbler/libstore"
	"github.com/cmu440/tribbler/rpc/storagerpc"
	"github.com/cmu440/tribbler/tests/proxycounter"
)

type testFunc struct {
	name string
	f    func()
}

var (
	portnum   = flag.Int("port", 9010, "port to listen on")
	testRegex = flag.String("t", "", "test to run")
)

var (
	pc         proxycounter.ProxyCounter
	ls         libstore.Libstore
	revokeConn *rpc.Client
	passCount  int
	failCount  int
)

var LOGE = log.New(os.Stderr, "", log.Lshortfile|log.Lmicroseconds)

// Initialize proxy and libstore
func initLibstore(storage, server, myhostport string, alwaysLease bool) (net.Listener, error) {
	l, err := net.Listen("tcp", server)
	if err != nil {
		LOGE.Println("Failed to listen:", err)
		return nil, err
	}

	// The ProxyServer acts like a "StorageServer" in the system, but also has some
	// additional functionalities that allow us to enforce the number of RPCs made
	// to the storage server, etc.
	proxyCounter, err := proxycounter.NewProxyCounter(storage, server)
	if err != nil {
		LOGE.Println("Failed to setup test:", err)
		return nil, err
	}
	pc = proxyCounter

	// Normally the StorageServer would register itself to receive RPCs,
	// but we don't call NewStorageServer here, do we need to do it here instead.
	rpc.RegisterName("StorageServer", storagerpc.Wrap(pc))

	// Normally the TribServer would call the two methods below when it is first
	// created, but these tests mock out the TribServer all together, so we do
	// it here instead.
	rpc.HandleHTTP()
	go http.Serve(l, nil)

	var leaseMode libstore.LeaseMode
	if alwaysLease {
		leaseMode = libstore.Always
	} else if myhostport == "" {
		leaseMode = libstore.Never
	} else {
		leaseMode = libstore.Normal
	}

	// Create and start the Libstore.
	libstore, err := libstore.NewLibstore(server, myhostport, leaseMode)
	if err != nil {
		LOGE.Println("Failed to create Libstore:", err)
		return nil, err
	}
	ls = libstore
	return l, nil
}

// Cleanup libstore and rpc hooks
func cleanupLibstore(l net.Listener) {
	// Close listener to stop http serve thread
	if l != nil {
		l.Close()
	}
	// Recreate default http serve mux
	http.DefaultServeMux = http.NewServeMux()
	// Recreate default rpc server
	rpc.DefaultServer = rpc.NewServer()
	// Unset libstore just in case
	ls = nil
}

// Force key into cache by requesting 2 * QUERY_CACHE_THRESH gets
func forceCacheGet(key string, value string) {
	ls.Put(key, value)
	for i := 0; i < 2*storagerpc.QueryCacheThresh; i++ {
		ls.Get(key)
	}
}

// Force key into cache by requesting 2 * QUERY_CACHE_THRESH get lists
func forceCacheGetList(key string, value string) {
	ls.AppendToList(key, value)
	for i := 0; i < 2*storagerpc.QueryCacheThresh; i++ {
		ls.GetList(key)
	}
}

// Revoke lease
func revokeLease(key string) (error, storagerpc.Status) {
	args := &storagerpc.RevokeLeaseArgs{Key: key}
	var reply storagerpc.RevokeLeaseReply
	err := revokeConn.Call("LeaseCallbacks.RevokeLease", args, &reply)
	return err, reply.Status
}

// Check rpc and byte count limits
func checkLimits(rpcCountLimit, byteCountLimit uint32) bool {
	if pc.GetRpcCount() > rpcCountLimit {
		LOGE.Println("FAIL: using too many RPCs")
		failCount++
		return true
	}
	if pc.GetByteCount() > byteCountLimit {
		LOGE.Println("FAIL: transferring too much data")
		failCount++
		return true
	}
	return false
}

// Check error
func checkError(err error, expectError bool) bool {
	if expectError {
		if err == nil {
			LOGE.Println("FAIL: error should be returned")
			failCount++
			return true
		}
	} else {
		if err != nil {
			LOGE.Println("FAIL: unexpected error returned:", err)
			failCount++
			return true
		}
	}
	return false
}

// Test libstore returns nil when it cannot connect to the server
func testNonexistentServer() {
	if l, err := libstore.NewLibstore(fmt.Sprintf("localhost:%d", *portnum), fmt.Sprintf("localhost:%d", *portnum), libstore.Normal); l == nil || err != nil {
		fmt.Println("PASS")
		passCount++
	} else {
		LOGE.Println("FAIL: libstore does not return a non-nil error when it cannot connect to nonexistent storage server")
		failCount++
	}
	cleanupLibstore(nil)
}

// Never request leases when myhostport is ""
func testNoLeases() {
	l, err := initLibstore(flag.Arg(0), fmt.Sprintf("localhost:%d", *portnum), "", false)
	if err != nil {
		LOGE.Println("FAIL:", err)
		failCount++
		return
	}
	defer cleanupLibstore(l)
	pc.Reset()
	forceCacheGet("key:", "value")
	if pc.GetLeaseRequestCount() > 0 {
		LOGE.Println("FAIL: should not request leases when myhostport is \"\"")
		failCount++
		return
	}
	fmt.Println("PASS")
	passCount++
}

// Always request leases when alwaysLease is true
func testAlwaysLeases() {
	l, err := initLibstore(flag.Arg(0), fmt.Sprintf("localhost:%d", *portnum), fmt.Sprintf("localhost:%d", *portnum), true)
	if err != nil {
		LOGE.Println("FAIL:", err)
		failCount++
		return
	}
	defer cleanupLibstore(l)
	pc.Reset()
	ls.Put("key:", "value")
	ls.Get("key:")
	if pc.GetLeaseRequestCount() == 0 {
		LOGE.Println("FAIL: should always request leases when alwaysLease is true")
		failCount++
		return
	}
	fmt.Println("PASS")
	passCount++
}

// Handle get error
func testGetError() {
	pc.Reset()
	pc.OverrideErr()
	defer pc.OverrideOff()
	_, err := ls.Get("key:1")
	if checkError(err, true) {
		return
	}
	if checkLimits(5, 50) {
		return
	}
	fmt.Println("PASS")
	passCount++
}

// Handle get error reply status
func testGetErrorStatus() {
	pc.Reset()
	pc.OverrideStatus(storagerpc.KeyNotFound)
	defer pc.OverrideOff()
	_, err := ls.Get("key:2")
	if checkError(err, true) {
		return
	}
	if checkLimits(5, 50) {
		return
	}
	fmt.Println("PASS")
	passCount++
}

// Handle valid get
func testGetValid() {
	ls.Put("key:3", "value")
	pc.Reset()
	v, err := ls.Get("key:3")
	if checkError(err, false) {
		return
	}
	if v != "value" {
		LOGE.Println("FAIL: got wrong value")
		failCount++
		return
	}
	if checkLimits(5, 50) {
		return
	}
	fmt.Println("PASS")
	passCount++
}

// Handle put error
func testPutError() {
	pc.Reset()
	pc.OverrideErr()
	defer pc.OverrideOff()
	err := ls.Put("key:4", "value")
	if checkError(err, true) {
		return
	}
	if checkLimits(5, 50) {
		return
	}
	fmt.Println("PASS")
	passCount++
}

// Handle put error reply status
func testPutErrorStatus() {
	pc.Reset()
	pc.OverrideStatus(storagerpc.WrongServer /* use arbitrary status */)
	defer pc.OverrideOff()
	err := ls.Put("key:5", "value")
	if checkError(err, true) {
		return
	}
	if checkLimits(5, 50) {
		return
	}
	fmt.Println("PASS")
	passCount++
}

// Handle valid put
func testPutValid() {
	pc.Reset()
	err := ls.Put("key:6", "value")
	if checkError(err, false) {
		return
	}
	if checkLimits(5, 50) {
		return
	}
	v, err := ls.Get("key:6")
	if checkError(err, false) {
		return
	}
	if v != "value" {
		LOGE.Println("FAIL: got wrong value")
		failCount++
		return
	}
	fmt.Println("PASS")
	passCount++
}

// Handle get list error
func testGetListError() {
	pc.Reset()
	pc.OverrideErr()
	defer pc.OverrideOff()
	_, err := ls.GetList("keylist:1")
	if checkError(err, true) {
		return
	}
	if checkLimits(5, 50) {
		return
	}
	fmt.Println("PASS")
	passCount++
}

// Handle get list error reply status
func testGetListErrorStatus() {
	pc.Reset()
	pc.OverrideStatus(storagerpc.ItemNotFound)
	defer pc.OverrideOff()
	_, err := ls.GetList("keylist:2")
	if checkError(err, true) {
		return
	}
	if checkLimits(5, 50) {
		return
	}
	fmt.Println("PASS")
	passCount++
}

// Handle valid get list
func testGetListValid() {
	ls.AppendToList("keylist:3", "value")
	pc.Reset()
	v, err := ls.GetList("keylist:3")
	if checkError(err, false) {
		return
	}
	if len(v) != 1 || v[0] != "value" {
		LOGE.Println("FAIL: got wrong value")
		failCount++
		return
	}
	if checkLimits(5, 50) {
		return
	}
	fmt.Println("PASS")
	passCount++
}

// Handle append to list error
func testAppendToListError() {
	pc.Reset()
	pc.OverrideErr()
	defer pc.OverrideOff()
	err := ls.AppendToList("keylist:4", "value")
	if checkError(err, true) {
		return
	}
	if checkLimits(5, 50) {
		return
	}
	fmt.Println("PASS")
	passCount++
}

// Handle append to list error reply status
func testAppendToListErrorStatus() {
	pc.Reset()
	pc.OverrideStatus(storagerpc.ItemExists)
	defer pc.OverrideOff()
	err := ls.AppendToList("keylist:5", "value")
	if checkError(err, true) {
		return
	}
	if checkLimits(5, 50) {
		return
	}
	fmt.Println("PASS")
	passCount++
}

// Handle valid append to list
func testAppendToListValid() {
	pc.Reset()
	err := ls.AppendToList("keylist:6", "value")
	if checkError(err, false) {
		return
	}
	if checkLimits(5, 50) {
		return
	}
	v, err := ls.GetList("keylist:6")
	if checkError(err, false) {
		return
	}
	if len(v) != 1 || v[0] != "value" {
		LOGE.Println("FAIL: got wrong value")
		failCount++
		return
	}
	fmt.Println("PASS")
	passCount++
}

// Handle remove from list error
func testRemoveFromListError() {
	pc.Reset()
	pc.OverrideErr()
	defer pc.OverrideOff()
	err := ls.RemoveFromList("keylist:7", "value")
	if checkError(err, true) {
		return
	}
	if checkLimits(5, 50) {
		return
	}
	fmt.Println("PASS")
	passCount++
}

// Handle remove from list error reply status
func testRemoveFromListErrorStatus() {
	pc.Reset()
	pc.OverrideStatus(storagerpc.ItemNotFound)
	defer pc.OverrideOff()
	err := ls.RemoveFromList("keylist:8", "value")
	if checkError(err, true) {
		return
	}
	if checkLimits(5, 50) {
		return
	}
	fmt.Println("PASS")
	passCount++
}

// Handle valid remove from list
func testRemoveFromListValid() {
	err := ls.AppendToList("keylist:9", "value1")
	if checkError(err, false) {
		return
	}
	err = ls.AppendToList("keylist:9", "value2")
	if checkError(err, false) {
		return
	}
	pc.Reset()
	err = ls.RemoveFromList("keylist:9", "value1")
	if checkError(err, false) {
		return
	}
	if checkLimits(5, 50) {
		return
	}
	v, err := ls.GetList("keylist:9")
	if checkError(err, false) {
		return
	}
	if len(v) != 1 || v[0] != "value2" {
		LOGE.Println("FAIL: got wrong value")
		failCount++
		return
	}
	fmt.Println("PASS")
	passCount++
}

// Cache < limit test for get
func testCacheGetLimit() {
	pc.Reset()
	ls.Put("keycacheget:1", "value")
	for i := 0; i < storagerpc.QueryCacheThresh-1; i++ {
		ls.Get("keycacheget:1")
	}
	if pc.GetLeaseRequestCount() > 0 {
		LOGE.Println("FAIL: should not request lease")
		failCount++
		return
	}
	fmt.Println("PASS")
	passCount++
}

// Cache > limit test for get
func testCacheGetLimit2() {
	pc.Reset()
	forceCacheGet("keycacheget:2", "value")
	if pc.GetLeaseRequestCount() == 0 {
		LOGE.Println("FAIL: should have requested lease")
		failCount++
		return
	}
	fmt.Println("PASS")
	passCount++
}

// Doesn't call server when using cache for get
func testCacheGetCorrect() {
	forceCacheGet("keycacheget:3", "value")
	pc.Reset()
	for i := 0; i < 100*storagerpc.QueryCacheThresh; i++ {
		v, err := ls.Get("keycacheget:3")
		if checkError(err, false) {
			return
		}
		if v != "value" {
			LOGE.Println("FAIL: got wrong value from cache")
			failCount++
			return
		}
	}
	if pc.GetRpcCount() > 0 {
		LOGE.Println("FAIL: should not contact server when using cache")
		failCount++
		return
	}
	fmt.Println("PASS")
	passCount++
}

// Cache respects granted flag for get
func testCacheGetLeaseNotGranted() {
	pc.DisableLease()
	defer pc.EnableLease()
	forceCacheGet("keycacheget:4", "value")
	pc.Reset()
	v, err := ls.Get("keycacheget:4")
	if checkError(err, false) {
		return
	}
	if v != "value" {
		LOGE.Println("FAIL: got wrong value")
		failCount++
		return
	}
	if pc.GetRpcCount() == 0 {
		LOGE.Println("FAIL: not respecting lease granted flag")
		failCount++
		return
	}
	fmt.Println("PASS")
	passCount++
}

// Cache requests leases until granted for get
func testCacheGetLeaseNotGranted2() {
	pc.DisableLease()
	defer pc.EnableLease()
	forceCacheGet("keycacheget:5", "value")
	pc.Reset()
	forceCacheGet("keycacheget:5", "value")
	if pc.GetLeaseRequestCount() == 0 {
		LOGE.Println("FAIL: not requesting leases after lease wasn't granted")
		failCount++
		return
	}
	fmt.Println("PASS")
	passCount++
}

// Cache respects lease timeout for get
func testCacheGetLeaseTimeout() {
	pc.OverrideLeaseSeconds(1)
	defer pc.OverrideLeaseSeconds(0)
	forceCacheGet("keycacheget:6", "value")
	time.Sleep(2 * time.Second)
	pc.Reset()
	v, err := ls.Get("keycacheget:6")
	if checkError(err, false) {
		return
	}
	if v != "value" {
		LOGE.Println("FAIL: got wrong value")
		failCount++
		return
	}
	if pc.GetRpcCount() == 0 {
		LOGE.Println("FAIL: not respecting lease timeout")
		failCount++
		return
	}
	fmt.Println("PASS")
	passCount++
}

// Cache memory leak for get
func testCacheGetMemoryLeak() {
	pc.OverrideLeaseSeconds(1)
	defer pc.OverrideLeaseSeconds(0)

	var memstats runtime.MemStats
	var initAlloc, finalAlloc uint64
	longValue := strings.Repeat("this sentence is 30 char long\n", 3000)

	// Run garbage collection and get memory stats.
	runtime.GC()
	runtime.ReadMemStats(&memstats)
	initAlloc = memstats.Alloc

	// Cache a lot of data.
	for i := 0; i < 1000; i++ {
		key := fmt.Sprintf("keymemleakget:%d", i)
		pc.Reset()
		forceCacheGet(key, longValue)
		if pc.GetLeaseRequestCount() == 0 {
			LOGE.Println("FAIL: not requesting leases")
			failCount++
			return
		}
		pc.Reset()
		v, err := ls.Get(key)
		if checkError(err, false) {
			return
		}
		if v != longValue {
			LOGE.Println("FAIL: got wrong value")
			failCount++
			return
		}
		if pc.GetRpcCount() > 0 {
			LOGE.Println("FAIL: not caching data")
			failCount++
			return
		}
	}

	runtime.GC()
	runtime.ReadMemStats(&memstats)

	// Wait for data to expire and someone to cleanup.
	time.Sleep(20 * time.Second)

	// Run garbage collection and get memory stats.
	runtime.GC()
	runtime.ReadMemStats(&memstats)
	finalAlloc = memstats.Alloc

	// The maximum number of bytes allowed to be allocated since the beginning
	// of this test until now (currently 5,000,000).
	const maxBytes = 5000000
	if finalAlloc < initAlloc || (finalAlloc-initAlloc) < maxBytes {
		fmt.Println("PASS")
		passCount++
	} else {
		LOGE.Printf("FAIL: Libstore not cleaning expired/cached data (bytes still in use: %d, max allowed: %d)\n",
			finalAlloc-initAlloc, maxBytes)
		failCount++
	}
}

// Revoke valid lease for get
func testRevokeGetValid() {
	forceCacheGet("keyrevokeget:1", "value")
	err, status := revokeLease("keyrevokeget:1")
	if checkError(err, false) {
		return
	}
	if status != storagerpc.OK {
		LOGE.Println("FAIL: revoke should return OK on success")
		failCount++
		return
	}
	pc.Reset()
	v, err := ls.Get("keyrevokeget:1")
	if checkError(err, false) {
		return
	}
	if v != "value" {
		LOGE.Println("FAIL: got wrong value")
		failCount++
		return
	}
	if pc.GetRpcCount() == 0 {
		LOGE.Println("FAIL: not respecting lease revoke")
		failCount++
		return
	}
	fmt.Println("PASS")
	passCount++
}

// Revoke nonexistent lease for get
func testRevokeGetNonexistent() {
	ls.Put("keyrevokeget:2", "value")
	// Just shouldn't die or cause future issues
	revokeLease("keyrevokeget:2")
	pc.Reset()
	v, err := ls.Get("keyrevokeget:2")
	if checkError(err, false) {
		return
	}
	if v != "value" {
		LOGE.Println("FAIL: got wrong value")
		failCount++
		return
	}
	if pc.GetRpcCount() == 0 {
		LOGE.Println("FAIL: should not be cached")
		failCount++
		return
	}
	fmt.Println("PASS")
	passCount++
}

// Revoke lease update for get
func testRevokeGetUpdate() {
	forceCacheGet("keyrevokeget:3", "value")
	pc.Reset()
	forceCacheGet("keyrevokeget:3", "value2")
	if pc.GetRpcCount() <= 1 || pc.GetLeaseRequestCount() == 0 {
		LOGE.Println("FAIL: not respecting lease revoke")
		failCount++
		return
	}
	pc.Reset()
	v, err := ls.Get("keyrevokeget:3")
	if checkError(err, false) {
		return
	}
	if v != "value2" {
		LOGE.Println("FAIL: got wrong value")
		failCount++
		return
	}
	if pc.GetRpcCount() > 0 {
		LOGE.Println("FAIL: should be cached")
		failCount++
		return
	}
	fmt.Println("PASS")
	passCount++
}

// Cache < limit test for get list
func testCacheGetListLimit() {
	pc.Reset()
	ls.AppendToList("keycachegetlist:1", "value")
	for i := 0; i < storagerpc.QueryCacheThresh-1; i++ {
		ls.GetList("keycachegetlist:1")
	}
	if pc.GetLeaseRequestCount() > 0 {
		LOGE.Println("FAIL: should not request lease")
		failCount++
		return
	}
	fmt.Println("PASS")
	passCount++
}

// Cache > limit test for get list
func testCacheGetListLimit2() {
	pc.Reset()
	forceCacheGetList("keycachegetlist:2", "value")
	if pc.GetLeaseRequestCount() == 0 {
		LOGE.Println("FAIL: should have requested lease")
		failCount++
		return
	}
	fmt.Println("PASS")
	passCount++
}

// Doesn't call server when using cache for get list
func testCacheGetListCorrect() {
	forceCacheGetList("keycachegetlist:3", "value")
	pc.Reset()
	for i := 0; i < 100*storagerpc.QueryCacheThresh; i++ {
		v, err := ls.GetList("keycachegetlist:3")
		if checkError(err, false) {
			return
		}
		if len(v) != 1 || v[0] != "value" {
			LOGE.Println("FAIL: got wrong value from cache")
			failCount++
			return
		}
	}
	if pc.GetRpcCount() > 0 {
		LOGE.Println("FAIL: should not contact server when using cache")
		failCount++
		return
	}
	fmt.Println("PASS")
	passCount++
}

// Cache respects granted flag for get list
func testCacheGetListLeaseNotGranted() {
	pc.DisableLease()
	defer pc.EnableLease()
	forceCacheGetList("keycachegetlist:4", "value")
	pc.Reset()
	v, err := ls.GetList("keycachegetlist:4")
	if checkError(err, false) {
		return
	}
	if len(v) != 1 || v[0] != "value" {
		LOGE.Println("FAIL: got wrong value")
		failCount++
		return
	}
	if pc.GetRpcCount() == 0 {
		LOGE.Println("FAIL: not respecting lease granted flag")
		failCount++
		return
	}
	fmt.Println("PASS")
	passCount++
}

// Cache requests leases until granted for get list
func testCacheGetListLeaseNotGranted2() {
	pc.DisableLease()
	defer pc.EnableLease()
	forceCacheGetList("keycachegetlist:5", "value")
	pc.Reset()
	forceCacheGetList("keycachegetlist:5", "value")
	if pc.GetLeaseRequestCount() == 0 {
		LOGE.Println("FAIL: not requesting leases after lease wasn't granted")
		failCount++
		return
	}
	fmt.Println("PASS")
	passCount++
}

// Cache respects lease timeout for get list
func testCacheGetListLeaseTimeout() {
	pc.OverrideLeaseSeconds(1)
	defer pc.OverrideLeaseSeconds(0)
	forceCacheGetList("keycachegetlist:6", "value")
	time.Sleep(2 * time.Second)
	pc.Reset()
	v, err := ls.GetList("keycachegetlist:6")
	if checkError(err, false) {
		return
	}
	if len(v) != 1 || v[0] != "value" {
		LOGE.Println("FAIL: got wrong value")
		failCount++
		return
	}
	if pc.GetRpcCount() == 0 {
		LOGE.Println("FAIL: not respecting lease timeout")
		failCount++
		return
	}
	fmt.Println("PASS")
	passCount++
}

// Cache memory leak for get list
func testCacheGetListMemoryLeak() {
	pc.OverrideLeaseSeconds(1)
	defer pc.OverrideLeaseSeconds(0)

	var memstats runtime.MemStats
	var initAlloc uint64
	var finalAlloc uint64
	longValue := strings.Repeat("this sentence is 30 char long\n", 3000)

	// Run garbage collection and get memory stats.
	runtime.GC()
	runtime.ReadMemStats(&memstats)
	initAlloc = memstats.Alloc

	// Cache a lot of data.
	for i := 0; i < 1000; i++ {
		key := fmt.Sprintf("keymemleakgetlist:%d", i)
		pc.Reset()
		forceCacheGetList(key, longValue)
		if pc.GetLeaseRequestCount() == 0 {
			LOGE.Println("FAIL: not requesting leases")
			failCount++
			return
		}
		pc.Reset()
		v, err := ls.GetList(key)
		if checkError(err, false) {
			return
		}
		if len(v) != 1 || v[0] != longValue {
			LOGE.Println("FAIL: got wrong value")
			failCount++
			return
		}
		if pc.GetRpcCount() > 0 {
			LOGE.Println("FAIL: not caching data")
			failCount++
			return
		}
	}

	runtime.GC()
	runtime.ReadMemStats(&memstats)

	// Wait for data to expire and someone to cleanup.
	time.Sleep(20 * time.Second)

	// Run garbage collection and get memory stats.
	runtime.GC()
	runtime.ReadMemStats(&memstats)
	finalAlloc = memstats.Alloc

	// The maximum number of bytes allowed to be allocated since the beginning
	// of this test until now (currently 5,000,000).
	const maxBytes = 5000000
	if finalAlloc < initAlloc || (finalAlloc-initAlloc) < maxBytes {
		fmt.Println("PASS")
		passCount++
	} else {
		LOGE.Printf("FAIL: Libstore not cleaning expired/cached data (bytes still in use: %d, max allowed: %d)\n",
			finalAlloc-initAlloc, maxBytes)
		failCount++
	}
}

// Revoke valid lease for get list
func testRevokeGetListValid() {
	forceCacheGetList("keyrevokegetlist:1", "value")
	err, status := revokeLease("keyrevokegetlist:1")
	if checkError(err, false) {
		return
	}
	if status != storagerpc.OK {
		LOGE.Println("FAIL: revoke should return OK on success")
		failCount++
		return
	}
	pc.Reset()
	v, err := ls.GetList("keyrevokegetlist:1")
	if checkError(err, false) {
		return
	}
	if len(v) != 1 || v[0] != "value" {
		LOGE.Println("FAIL: got wrong value")
		failCount++
		return
	}
	if pc.GetRpcCount() == 0 {
		LOGE.Println("FAIL: not respecting lease revoke")
		failCount++
		return
	}
	fmt.Println("PASS")
	passCount++
}

// Revoke nonexistent lease for get list
func testRevokeGetListNonexistent() {
	ls.AppendToList("keyrevokegetlist:2", "value")
	// Just shouldn't die or cause future issues
	revokeLease("keyrevokegetlist:2")
	pc.Reset()
	v, err := ls.GetList("keyrevokegetlist:2")
	if checkError(err, false) {
		return
	}
	if len(v) != 1 || v[0] != "value" {
		LOGE.Println("FAIL: got wrong value")
		failCount++
		return
	}
	if pc.GetRpcCount() == 0 {
		LOGE.Println("FAIL: should not be cached")
		failCount++
		return
	}
	fmt.Println("PASS")
	passCount++
}

// Revoke lease update for get list
func testRevokeGetListUpdate() {
	forceCacheGetList("keyrevokegetlist:3", "value")
	ls.RemoveFromList("keyrevokegetlist:3", "value")
	pc.Reset()
	forceCacheGetList("keyrevokegetlist:3", "value2")
	if pc.GetRpcCount() <= 1 || pc.GetLeaseRequestCount() == 0 {
		LOGE.Println("FAIL: not respecting lease revoke")
		failCount++
		return
	}
	pc.Reset()
	v, err := ls.GetList("keyrevokegetlist:3")
	if checkError(err, false) {
		return
	}
	if len(v) != 1 || v[0] != "value2" {
		LOGE.Println("FAIL: got wrong value")
		failCount++
		return
	}
	if pc.GetRpcCount() > 0 {
		LOGE.Println("FAIL: should be cached")
		failCount++
		return
	}
	fmt.Println("PASS")
	passCount++
}

func main() {
	initTests := []testFunc{
		{"testNonexistentServer", testNonexistentServer},
		{"testNoLeases", testNoLeases},
		{"testAlwaysLeases", testAlwaysLeases},
	}
	tests := []testFunc{
		{"testGetError", testGetError},
		{"testGetErrorStatus", testGetErrorStatus},
		{"testGetValid", testGetValid},
		{"testPutError", testPutError},
		{"testPutErrorStatus", testPutErrorStatus},
		{"testPutValid", testPutValid},
		{"testGetListError", testGetListError},
		{"testGetListErrorStatus", testGetListErrorStatus},
		{"testGetListValid", testGetListValid},
		{"testAppendToListError", testAppendToListError},
		{"testAppendToListErrorStatus", testAppendToListErrorStatus},
		{"testAppendToListValid", testAppendToListValid},
		{"testRemoveFromListError", testRemoveFromListError},
		{"testRemoveFromListErrorStatus", testRemoveFromListErrorStatus},
		{"testRemoveFromListValid", testRemoveFromListValid},
		{"testCacheGetLimit", testCacheGetLimit},
		{"testCacheGetLimit2", testCacheGetLimit2},
		{"testCacheGetCorrect", testCacheGetCorrect},
		{"testCacheGetLeaseNotGranted", testCacheGetLeaseNotGranted},
		{"testCacheGetLeaseNotGranted2", testCacheGetLeaseNotGranted2},
		{"testCacheGetLeaseTimeout", testCacheGetLeaseTimeout},
		{"testCacheGetMemoryLeak", testCacheGetMemoryLeak},
		{"testRevokeGetValid", testRevokeGetValid},
		{"testRevokeGetNonexistent", testRevokeGetNonexistent},
		{"testRevokeGetUpdate", testRevokeGetUpdate},
		{"testCacheGetListLimit", testCacheGetListLimit},
		{"testCacheGetListLimit2", testCacheGetListLimit2},
		{"testCacheGetListCorrect", testCacheGetListCorrect},
		{"testCacheGetListLeaseNotGranted", testCacheGetListLeaseNotGranted},
		{"testCacheGetListLeaseNotGranted2", testCacheGetListLeaseNotGranted2},
		{"testCacheGetListLeaseTimeout", testCacheGetListLeaseTimeout},
		{"testCacheGetListMemoryLeak", testCacheGetListMemoryLeak},
		{"testRevokeGetListValid", testRevokeGetListValid},
		{"testRevokeGetListNonexistent", testRevokeGetListNonexistent},
		{"testRevokeGetListUpdate", testRevokeGetListUpdate},
	}

	flag.Parse()
	if flag.NArg() < 1 {
		LOGE.Fatalln("Usage: libtest <storage master host:port>")
	}

	var err error

	// Run init tests
	for _, t := range initTests {
		if b, err := regexp.MatchString(*testRegex, t.name); b && err == nil {
			fmt.Printf("Running %s:\n", t.name)
			t.f()
		}
		// Give the current Listener some time to close before creating
		// a new Libstore.
		time.Sleep(time.Duration(500) * time.Millisecond)
	}

	_, err = initLibstore(flag.Arg(0), fmt.Sprintf("localhost:%d", *portnum), fmt.Sprintf("localhost:%d", *portnum), false)
	if err != nil {
		return
	}
	revokeConn, err = rpc.DialHTTP("tcp", fmt.Sprintf("localhost:%d", *portnum))
	if err != nil {
		LOGE.Println("Failed to connect to Libstore RPC:", err)
		return
	}

	// Run tests
	for _, t := range tests {
		if b, err := regexp.MatchString(*testRegex, t.name); b && err == nil {
			fmt.Printf("Running %s:\n", t.name)
			t.f()
		}
	}

	fmt.Printf("Passed (%d/%d) tests\n", passCount, passCount+failCount)
}
