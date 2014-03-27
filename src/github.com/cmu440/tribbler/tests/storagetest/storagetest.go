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
	"time"

	"github.com/cmu440/tribbler/rpc/librpc"
	"github.com/cmu440/tribbler/rpc/storagerpc"
)

type storageTester struct {
	srv        *rpc.Client
	myhostport string
	recvRevoke map[string]bool // whether we have received a RevokeLease for key x
	compRevoke map[string]bool // whether we have replied the RevokeLease for key x
	delay      float32         // how long to delay the reply of RevokeLease
}

type testFunc struct {
	name string
	f    func()
}

var (
	portnum   = flag.Int("port", 9019, "port # to listen on")
	testType  = flag.Int("type", 1, "type of test, 1: jtest, 2: btest")
	numServer = flag.Int("N", 1, "(jtest only) total # of storage servers")
	myID      = flag.Int("id", 1, "(jtest only) my id")
	testRegex = flag.String("t", "", "test to run")
	passCount int
	failCount int
	st        *storageTester
)

var LOGE = log.New(os.Stderr, "", log.Lshortfile|log.Lmicroseconds)

var statusMap = map[storagerpc.Status]string{
	storagerpc.OK:           "OK",
	storagerpc.KeyNotFound:  "KeyNotFound",
	storagerpc.ItemNotFound: "ItemNotFound",
	storagerpc.WrongServer:  "WrongServer",
	storagerpc.ItemExists:   "ItemExists",
	storagerpc.NotReady:     "NotReady",
	0:                       "Unknown",
}

func initStorageTester(server, myhostport string) (*storageTester, error) {
	tester := new(storageTester)
	tester.myhostport = myhostport
	tester.recvRevoke = make(map[string]bool)
	tester.compRevoke = make(map[string]bool)

	// Create RPC connection to storage server.
	srv, err := rpc.DialHTTP("tcp", server)
	if err != nil {
		return nil, fmt.Errorf("could not connect to server %s", server)
	}

	rpc.RegisterName("LeaseCallbacks", librpc.Wrap(tester))
	rpc.HandleHTTP()

	l, err := net.Listen("tcp", fmt.Sprintf(":%d", *portnum))
	if err != nil {
		LOGE.Fatalln("Failed to listen:", err)
	}
	go http.Serve(l, nil)
	tester.srv = srv
	return tester, nil
}

func (st *storageTester) ResetDelay() {
	st.delay = 0
}

func (st *storageTester) SetDelay(f float32) {
	st.delay = f * (storagerpc.LeaseSeconds + storagerpc.LeaseGuardSeconds)
}

func (st *storageTester) RevokeLease(args *storagerpc.RevokeLeaseArgs, reply *storagerpc.RevokeLeaseReply) error {
	st.recvRevoke[args.Key] = true
	st.compRevoke[args.Key] = false
	time.Sleep(time.Duration(st.delay*1000) * time.Millisecond)
	st.compRevoke[args.Key] = true
	reply.Status = storagerpc.OK
	return nil
}

func (st *storageTester) RegisterServer() (*storagerpc.RegisterReply, error) {
	node := storagerpc.Node{HostPort: st.myhostport, NodeID: uint32(*myID)}
	args := &storagerpc.RegisterArgs{ServerInfo: node}
	var reply storagerpc.RegisterReply
	err := st.srv.Call("StorageServer.RegisterServer", args, &reply)
	return &reply, err
}

func (st *storageTester) GetServers() (*storagerpc.GetServersReply, error) {
	args := &storagerpc.GetServersArgs{}
	var reply storagerpc.GetServersReply
	err := st.srv.Call("StorageServer.GetServers", args, &reply)
	return &reply, err
}

func (st *storageTester) Put(key, value string) (*storagerpc.PutReply, error) {
	args := &storagerpc.PutArgs{Key: key, Value: value}
	var reply storagerpc.PutReply
	err := st.srv.Call("StorageServer.Put", args, &reply)
	return &reply, err
}

func (st *storageTester) Get(key string, wantlease bool) (*storagerpc.GetReply, error) {
	args := &storagerpc.GetArgs{Key: key, WantLease: wantlease, HostPort: st.myhostport}
	var reply storagerpc.GetReply
	err := st.srv.Call("StorageServer.Get", args, &reply)
	return &reply, err
}

func (st *storageTester) GetList(key string, wantlease bool) (*storagerpc.GetListReply, error) {
	args := &storagerpc.GetArgs{Key: key, WantLease: wantlease, HostPort: st.myhostport}
	var reply storagerpc.GetListReply
	err := st.srv.Call("StorageServer.GetList", args, &reply)
	return &reply, err
}

func (st *storageTester) RemoveFromList(key, removeitem string) (*storagerpc.PutReply, error) {
	args := &storagerpc.PutArgs{Key: key, Value: removeitem}
	var reply storagerpc.PutReply
	err := st.srv.Call("StorageServer.RemoveFromList", args, &reply)
	return &reply, err
}

func (st *storageTester) AppendToList(key, newitem string) (*storagerpc.PutReply, error) {
	args := &storagerpc.PutArgs{Key: key, Value: newitem}
	var reply storagerpc.PutReply
	err := st.srv.Call("StorageServer.AppendToList", args, &reply)
	return &reply, err
}

// Check error and status
func checkErrorStatus(err error, status, expectedStatus storagerpc.Status) bool {
	if err != nil {
		LOGE.Println("FAIL: unexpected error returned:", err)
		failCount++
		return true
	}
	if status != expectedStatus {
		LOGE.Printf("FAIL: incorrect status %s, expected status %s\n", statusMap[status], statusMap[expectedStatus])
		failCount++
		return true
	}
	return false
}

// Check error
func checkError(err error, expectError bool) bool {
	if expectError {
		if err == nil {
			LOGE.Println("FAIL: non-nil error should be returned")
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

// Check list
func checkList(list []string, expectedList []string) bool {
	if len(list) != len(expectedList) {
		LOGE.Printf("FAIL: incorrect list %v, expected list %v\n", list, expectedList)
		failCount++
		return true
	}
	m := make(map[string]bool)
	for _, s := range list {
		m[s] = true
	}
	for _, s := range expectedList {
		if m[s] == false {
			LOGE.Printf("FAIL: incorrect list %v, expected list %v\n", list, expectedList)
			failCount++
			return true
		}
	}
	return false
}

// We treat a RPC call finihsed in 0.5 seconds as OK
func isTimeOK(d time.Duration) bool {
	return d < 500*time.Millisecond
}

// Cache a key
func cacheKey(key string) bool {
	replyP, err := st.Put(key, "old-value")
	if checkErrorStatus(err, replyP.Status, storagerpc.OK) {
		return true
	}

	// get and cache key
	replyG, err := st.Get(key, true)
	if checkErrorStatus(err, replyG.Status, storagerpc.OK) {
		return true
	}
	if !replyG.Lease.Granted {
		LOGE.Println("FAIL: Failed to get lease")
		failCount++
		return true
	}
	return false
}

// Cache a list key
func cacheKeyList(key string) bool {
	replyP, err := st.AppendToList(key, "old-value")
	if checkErrorStatus(err, replyP.Status, storagerpc.OK) {
		return true
	}

	// get and cache key
	replyL, err := st.GetList(key, true)
	if checkErrorStatus(err, replyL.Status, storagerpc.OK) {
		return true
	}
	if !replyL.Lease.Granted {
		LOGE.Println("FAIL: Failed to get lease")
		failCount++
		return true
	}
	return false
}

/////////////////////////////////////////////
//  test storage server initialization
/////////////////////////////////////////////

// make sure to run N-1 servers in shell before entering this function
func testInitStorageServers() {
	// test get server
	replyGS, err := st.GetServers()
	if checkError(err, false) {
		return
	}
	if replyGS.Status == storagerpc.OK {
		LOGE.Println("FAIL: storage system should not be ready:", err)
		failCount++
		return
	}

	// test register
	replyR, err := st.RegisterServer()
	if checkError(err, false) {
		return
	}
	if replyR.Status != storagerpc.OK || replyR.Servers == nil {
		LOGE.Println("FAIL: storage system should be ready and Servers field should be non-nil:", err)
		failCount++
		return
	}
	if len(replyR.Servers) != (*numServer) {
		LOGE.Println("FAIL: storage system returned wrong server list:", err)
		failCount++
		return
	}

	// test key range
	replyG, err := st.Get("wrongkey:1", false)
	if checkErrorStatus(err, replyG.Status, storagerpc.WrongServer) {
		return
	}
	fmt.Println("PASS")
	passCount++
}

/////////////////////////////////////////////
//  test basic storage operations
/////////////////////////////////////////////

// Get keys without and with wantlease
func testPutGet() {
	// get an invalid key
	replyG, err := st.Get("nullkey:1", false)
	if checkErrorStatus(err, replyG.Status, storagerpc.KeyNotFound) {
		return
	}

	replyP, err := st.Put("keyputget:1", "value")
	if checkErrorStatus(err, replyP.Status, storagerpc.OK) {
		return
	}

	// without asking for a lease
	replyG, err = st.Get("keyputget:1", false)
	if checkErrorStatus(err, replyG.Status, storagerpc.OK) {
		return
	}
	if replyG.Value != "value" {
		LOGE.Println("FAIL: got wrong value")
		failCount++
		return
	}
	if replyG.Lease.Granted {
		LOGE.Println("FAIL: did not apply for lease")
		failCount++
		return
	}

	// now I want a lease this time
	replyG, err = st.Get("keyputget:1", true)
	if checkErrorStatus(err, replyG.Status, storagerpc.OK) {
		return
	}
	if replyG.Value != "value" {
		LOGE.Println("FAIL: got wrong value")
		failCount++
		return
	}
	if !replyG.Lease.Granted {
		LOGE.Println("FAIL: did not get lease")
		failCount++
		return
	}

	fmt.Println("PASS")
	passCount++
}

// list related operations
func testAppendGetRemoveList() {
	// test AppendToList
	replyP, err := st.AppendToList("keylist:1", "value1")
	if checkErrorStatus(err, replyP.Status, storagerpc.OK) {
		return
	}

	// test GetList
	replyL, err := st.GetList("keylist:1", false)
	if checkErrorStatus(err, replyL.Status, storagerpc.OK) {
		return
	}
	if len(replyL.Value) != 1 || replyL.Value[0] != "value1" {
		LOGE.Println("FAIL: got wrong value")
		failCount++
		return
	}

	// test AppendToList for a duplicated item
	replyP, err = st.AppendToList("keylist:1", "value1")
	if checkErrorStatus(err, replyP.Status, storagerpc.ItemExists) {
		return
	}

	// test AppendToList for a different item
	replyP, err = st.AppendToList("keylist:1", "value2")
	if checkErrorStatus(err, replyP.Status, storagerpc.OK) {
		return
	}

	// test RemoveFromList for the first item
	replyP, err = st.RemoveFromList("keylist:1", "value1")
	if checkErrorStatus(err, replyP.Status, storagerpc.OK) {
		return
	}

	// test RemoveFromList for removed item
	replyP, err = st.RemoveFromList("keylist:1", "value1")
	if checkErrorStatus(err, replyP.Status, storagerpc.ItemNotFound) {
		return
	}

	// test GetList after RemoveFromList
	replyL, err = st.GetList("keylist:1", false)
	if checkErrorStatus(err, replyL.Status, storagerpc.OK) {
		return
	}
	if len(replyL.Value) != 1 || replyL.Value[0] != "value2" {
		LOGE.Println("FAIL: got wrong value")
		failCount++
		return
	}

	fmt.Println("PASS")
	passCount++
}

/////////////////////////////////////////////
//  test revoke related
/////////////////////////////////////////////

// Without leasing, we should not expect revoke
func testUpdateWithoutLease() {
	key := "revokekey:0"

	replyP, err := st.Put(key, "value")
	if checkErrorStatus(err, replyP.Status, storagerpc.OK) {
		return
	}

	// get without caching this item
	replyG, err := st.Get(key, false)
	if checkErrorStatus(err, replyG.Status, storagerpc.OK) {
		return
	}

	// update this key
	replyP, err = st.Put(key, "value1")
	if checkErrorStatus(err, replyP.Status, storagerpc.OK) {
		return
	}

	// get without caching this item
	replyG, err = st.Get(key, false)
	if checkErrorStatus(err, replyG.Status, storagerpc.OK) {
		return
	}

	if st.recvRevoke[key] {
		LOGE.Println("FAIL: expect no revoke")
		failCount++
		return
	}

	fmt.Println("PASS")
	passCount++
}

// updating a key before its lease expires
// expect a revoke msg from storage server
func testUpdateBeforeLeaseExpire() {
	key := "revokekey:1"

	if cacheKey(key) {
		return
	}

	// update this key
	replyP, err := st.Put(key, "value1")
	if checkErrorStatus(err, replyP.Status, storagerpc.OK) {
		return
	}

	// read it back
	replyG, err := st.Get(key, false)
	if checkErrorStatus(err, replyG.Status, storagerpc.OK) {
		return
	}
	if replyG.Value != "value1" {
		LOGE.Println("FAIL: got wrong value")
		failCount++
		return
	}

	// expect a revoke msg, check if we receive it
	if !st.recvRevoke[key] {
		LOGE.Println("FAIL: did not receive revoke")
		failCount++
		return
	}

	fmt.Println("PASS")
	passCount++
}

// updating a key after its lease expires
// expect no revoke msg received from storage server
func testUpdateAfterLeaseExpire() {
	key := "revokekey:2"

	if cacheKey(key) {
		return
	}

	// sleep until lease expires
	time.Sleep((storagerpc.LeaseSeconds + storagerpc.LeaseGuardSeconds + 1) * time.Second)

	// update this key
	replyP, err := st.Put(key, "value1")
	if checkErrorStatus(err, replyP.Status, storagerpc.OK) {
		return
	}

	// read back this item
	replyG, err := st.Get(key, false)
	if checkErrorStatus(err, replyG.Status, storagerpc.OK) {
		return
	}
	if replyG.Value != "value1" {
		LOGE.Println("FAIL: got wrong value")
		failCount++
		return
	}

	// expect no revoke msg, check if we receive any
	if st.recvRevoke[key] {
		LOGE.Println("FAIL: should not receive revoke")
		failCount++
		return
	}

	fmt.Println("PASS")
	passCount++
}

// helper function for delayed revoke tests
func delayedRevoke(key string, f func() bool) bool {
	if cacheKey(key) {
		return true
	}

	// trigger a delayed revocation in background
	var replyP *storagerpc.PutReply
	var err error
	putCh := make(chan bool)
	doneCh := make(chan bool)
	go func() {
		// put key1 again to trigger a revoke
		replyP, err = st.Put(key, "new-value")
		putCh <- true
	}()
	// ensure Put has gotten to server
	time.Sleep(100 * time.Millisecond)

	// run rest of function in go routine to allow for timeouts
	go func() {
		// run rest of test function
		ret := f()
		// wait for put to complete
		<-putCh
		// check for failures
		if ret {
			doneCh <- true
			return
		}
		if checkErrorStatus(err, replyP.Status, storagerpc.OK) {
			doneCh <- true
			return
		}
		doneCh <- false
	}()

	// wait for test completion or timeout
	select {
	case ret := <-doneCh:
		return ret
	case <-time.After((storagerpc.LeaseSeconds + storagerpc.LeaseGuardSeconds + 1) * time.Second):
		break
	}
	LOGE.Println("FAIL: timeout, may erroneously increase test count")
	failCount++
	return true
}

// when revoking leases for key "x",
// storage server should not block queries for other keys
func testDelayedRevokeWithoutBlocking() {
	st.SetDelay(0.5)
	defer st.ResetDelay()

	key1 := "revokekey:3"
	key2 := "revokekey:4"

	// function called during revoke of key1
	f := func() bool {
		ts := time.Now()
		// put key2, this should not block
		replyP, err := st.Put(key2, "value")
		if checkErrorStatus(err, replyP.Status, storagerpc.OK) {
			return true
		}
		if !isTimeOK(time.Since(ts)) {
			LOGE.Println("FAIL: concurrent Put got blocked")
			failCount++
			return true
		}

		ts = time.Now()
		// get key2, this should not block
		replyG, err := st.Get(key2, false)
		if checkErrorStatus(err, replyG.Status, storagerpc.OK) {
			return true
		}
		if replyG.Value != "value" {
			LOGE.Println("FAIL: get got wrong value")
			failCount++
			return true
		}
		if !isTimeOK(time.Since(ts)) {
			LOGE.Println("FAIL: concurrent Get got blocked")
			failCount++
			return true
		}
		return false
	}

	if delayedRevoke(key1, f) {
		return
	}
	fmt.Println("PASS")
	passCount++
}

// when revoking leases for key "x",
// storage server should stop leasing for "x"
// before revoking completes or old lease expires.
// this function tests the former case
func testDelayedRevokeWithLeaseRequest1() {
	st.SetDelay(0.5) // Revoke finishes before lease expires
	defer st.ResetDelay()

	key1 := "revokekey:5"

	// function called during revoke of key1
	f := func() bool {
		ts := time.Now()
		// get key1 and want a lease
		replyG, err := st.Get(key1, true)
		if checkErrorStatus(err, replyG.Status, storagerpc.OK) {
			return true
		}
		if isTimeOK(time.Since(ts)) {
			// in this case, server should reply old value and refuse lease
			if replyG.Lease.Granted || replyG.Value != "old-value" {
				LOGE.Println("FAIL: server should return old value and not grant lease")
				failCount++
				return true
			}
		} else {
			if !st.compRevoke[key1] || (!replyG.Lease.Granted || replyG.Value != "new-value") {
				LOGE.Println("FAIL: server should return new value and grant lease")
				failCount++
				return true
			}
		}
		return false
	}

	if delayedRevoke(key1, f) {
		return
	}
	fmt.Println("PASS")
	passCount++
}

// when revoking leases for key "x",
// storage server should stop leasing for "x"
// before revoking completes or old lease expires.
// this function tests the latter case
// The diff from the previous test is
// st.compRevoke[key1] in the else case
func testDelayedRevokeWithLeaseRequest2() {
	st.SetDelay(2) // Lease expires before revoking finishes
	defer st.ResetDelay()

	key1 := "revokekey:15"

	// function called during revoke of key1
	f := func() bool {
		ts := time.Now()
		// get key1 and want a lease
		replyG, err := st.Get(key1, true)
		if checkErrorStatus(err, replyG.Status, storagerpc.OK) {
			return true
		}
		if isTimeOK(time.Since(ts)) {
			// in this case, server should reply old value and refuse lease
			if replyG.Lease.Granted || replyG.Value != "old-value" {
				LOGE.Println("FAIL: server should return old value and not grant lease")
				failCount++
				return true
			}
		} else {
			if st.compRevoke[key1] || (!replyG.Lease.Granted || replyG.Value != "new-value") {
				LOGE.Println("FAIL: server should return new value and grant lease")
				failCount++
				return true
			}
		}
		return false
	}

	if delayedRevoke(key1, f) {
		return
	}
	fmt.Println("PASS")
	passCount++
}

// when revoking leases for key "x",
// storage server should hold upcoming updates for "x",
// until either all revocations complete or the lease expires
// this function tests the former case
func testDelayedRevokeWithUpdate1() {
	st.SetDelay(0.5) // revocation takes longer, but still completes before lease expires
	defer st.ResetDelay()

	key1 := "revokekey:6"

	// function called during revoke of key1
	f := func() bool {
		// put key1, this should block
		replyP, err := st.Put(key1, "newnew-value")
		if checkErrorStatus(err, replyP.Status, storagerpc.OK) {
			return true
		}
		if !st.compRevoke[key1] {
			LOGE.Println("FAIL: storage server should hold modification to key x during finishing revocating all lease holders of x")
			failCount++
			return true
		}
		replyG, err := st.Get(key1, false)
		if checkErrorStatus(err, replyG.Status, storagerpc.OK) {
			return true
		}
		if replyG.Value != "newnew-value" {
			LOGE.Println("FAIL: got wrong value")
			failCount++
			return true
		}
		return false
	}

	if delayedRevoke(key1, f) {
		return
	}
	fmt.Println("PASS")
	passCount++
}

// when revoking leases for key "x",
// storage server should hold upcoming updates for "x",
// until either all revocations complete or the lease expires
// this function tests the latter case
func testDelayedRevokeWithUpdate2() {
	st.SetDelay(2) // lease expires before revocation completes
	defer st.ResetDelay()

	key1 := "revokekey:7"

	// function called during revoke of key1
	f := func() bool {
		ts := time.Now()
		// put key1, this should block
		replyP, err := st.Put(key1, "newnew-value")
		if checkErrorStatus(err, replyP.Status, storagerpc.OK) {
			return true
		}
		d := time.Since(ts)
		if d < (storagerpc.LeaseSeconds+storagerpc.LeaseGuardSeconds-1)*time.Second {
			LOGE.Println("FAIL: storage server should hold this Put until leases expires key1")
			failCount++
			return true
		}
		if st.compRevoke[key1] {
			LOGE.Println("FAIL: storage server should not block this Put till the lease revoke of key1")
			failCount++
			return true
		}
		replyG, err := st.Get(key1, false)
		if checkErrorStatus(err, replyG.Status, storagerpc.OK) {
			return true
		}
		if replyG.Value != "newnew-value" {
			LOGE.Println("FAIL: got wrong value")
			failCount++
			return true
		}
		return false
	}

	if delayedRevoke(key1, f) {
		return
	}
	fmt.Println("PASS")
	passCount++
}

// remote libstores may not even reply all RevokeLease RPC calls.
// in this case, service should continue after lease expires
func testDelayedRevokeWithUpdate3() {
	st.SetDelay(2) // lease expires before revocation completes
	defer st.ResetDelay()

	key1 := "revokekey:8"

	// function called during revoke of key1
	f := func() bool {
		// sleep here until lease expires on the remote server
		time.Sleep((storagerpc.LeaseSeconds + storagerpc.LeaseGuardSeconds) * time.Second)

		// put key1, this should not block
		ts := time.Now()
		replyP, err := st.Put(key1, "newnew-value")
		if checkErrorStatus(err, replyP.Status, storagerpc.OK) {
			return true
		}
		if !isTimeOK(time.Since(ts)) {
			LOGE.Println("FAIL: storage server should not block this Put")
			failCount++
			return true
		}
		// get key1 and want lease, this should not block
		ts = time.Now()
		replyG, err := st.Get(key1, true)
		if checkErrorStatus(err, replyG.Status, storagerpc.OK) {
			return true
		}
		if replyG.Value != "newnew-value" {
			LOGE.Println("FAIL: got wrong value")
			failCount++
			return true
		}
		if !isTimeOK(time.Since(ts)) {
			LOGE.Println("FAIL: storage server should not block this Get")
			failCount++
			return true
		}
		return false
	}

	if delayedRevoke(key1, f) {
		return
	}
	fmt.Println("PASS")
	passCount++
}

// Without leasing, we should not expect revoke
func testUpdateListWithoutLease() {
	key := "revokelistkey:0"

	replyP, err := st.AppendToList(key, "value")
	if checkErrorStatus(err, replyP.Status, storagerpc.OK) {
		return
	}

	// get without caching this item
	replyL, err := st.GetList(key, false)
	if checkErrorStatus(err, replyL.Status, storagerpc.OK) {
		return
	}

	// update this key
	replyP, err = st.AppendToList(key, "value1")
	if checkErrorStatus(err, replyP.Status, storagerpc.OK) {
		return
	}
	replyP, err = st.RemoveFromList(key, "value1")
	if checkErrorStatus(err, replyP.Status, storagerpc.OK) {
		return
	}

	// get without caching this item
	replyL, err = st.GetList(key, false)
	if checkErrorStatus(err, replyL.Status, storagerpc.OK) {
		return
	}

	if st.recvRevoke[key] {
		LOGE.Println("FAIL: expect no revoke")
		failCount++
		return
	}

	fmt.Println("PASS")
	passCount++
}

// updating a key before its lease expires
// expect a revoke msg from storage server
func testUpdateListBeforeLeaseExpire() {
	key := "revokelistkey:1"

	if cacheKeyList(key) {
		return
	}

	// update this key
	replyP, err := st.AppendToList(key, "value1")
	if checkErrorStatus(err, replyP.Status, storagerpc.OK) {
		return
	}
	replyP, err = st.RemoveFromList(key, "value1")
	if checkErrorStatus(err, replyP.Status, storagerpc.OK) {
		return
	}

	// read it back
	replyL, err := st.GetList(key, false)
	if checkErrorStatus(err, replyL.Status, storagerpc.OK) {
		return
	}
	if len(replyL.Value) != 1 || replyL.Value[0] != "old-value" {
		LOGE.Println("FAIL: got wrong value")
		failCount++
		return
	}

	// expect a revoke msg, check if we receive it
	if !st.recvRevoke[key] {
		LOGE.Println("FAIL: did not receive revoke")
		failCount++
		return
	}

	fmt.Println("PASS")
	passCount++
}

// updating a key after its lease expires
// expect no revoke msg received from storage server
func testUpdateListAfterLeaseExpire() {
	key := "revokelistkey:2"

	if cacheKeyList(key) {
		return
	}

	// sleep until lease expires
	time.Sleep((storagerpc.LeaseSeconds + storagerpc.LeaseGuardSeconds + 1) * time.Second)

	// update this key
	replyP, err := st.AppendToList(key, "value1")
	if checkErrorStatus(err, replyP.Status, storagerpc.OK) {
		return
	}
	replyP, err = st.RemoveFromList(key, "value1")
	if checkErrorStatus(err, replyP.Status, storagerpc.OK) {
		return
	}

	// read back this item
	replyL, err := st.GetList(key, false)
	if checkErrorStatus(err, replyL.Status, storagerpc.OK) {
		return
	}
	if len(replyL.Value) != 1 || replyL.Value[0] != "old-value" {
		LOGE.Println("FAIL: got wrong value")
		failCount++
		return
	}

	// expect no revoke msg, check if we receive any
	if st.recvRevoke[key] {
		LOGE.Println("FAIL: should not receive revoke")
		failCount++
		return
	}

	fmt.Println("PASS")
	passCount++
}

// helper function for delayed revoke tests
func delayedRevokeList(key string, f func() bool) bool {
	if cacheKeyList(key) {
		return true
	}

	// trigger a delayed revocation in background
	var replyP *storagerpc.PutReply
	var err error
	appendCh := make(chan bool)
	doneCh := make(chan bool)
	go func() {
		// append key to trigger a revoke
		replyP, err = st.AppendToList(key, "new-value")
		appendCh <- true
	}()
	// ensure Put has gotten to server
	time.Sleep(100 * time.Millisecond)

	// run rest of function in go routine to allow for timeouts
	go func() {
		// run rest of test function
		ret := f()
		// wait for append to complete
		<-appendCh
		// check for failures
		if ret {
			doneCh <- true
			return
		}
		if checkErrorStatus(err, replyP.Status, storagerpc.OK) {
			doneCh <- true
			return
		}
		doneCh <- false
	}()

	// wait for test completion or timeout
	select {
	case ret := <-doneCh:
		return ret
	case <-time.After((storagerpc.LeaseSeconds + storagerpc.LeaseGuardSeconds + 1) * time.Second):
		break
	}
	LOGE.Println("FAIL: timeout, may erroneously increase test count")
	failCount++
	return true
}

// when revoking leases for key "x",
// storage server should not block queries for other keys
func testDelayedRevokeListWithoutBlocking() {
	st.SetDelay(0.5)
	defer st.ResetDelay()

	key1 := "revokelistkey:3"
	key2 := "revokelistkey:4"

	// function called during revoke of key1
	f := func() bool {
		ts := time.Now()
		// put key2, this should not block
		replyP, err := st.AppendToList(key2, "value")
		if checkErrorStatus(err, replyP.Status, storagerpc.OK) {
			return true
		}
		if !isTimeOK(time.Since(ts)) {
			LOGE.Println("FAIL: concurrent Append got blocked")
			failCount++
			return true
		}

		ts = time.Now()
		// get key2, this should not block
		replyL, err := st.GetList(key2, false)
		if checkErrorStatus(err, replyL.Status, storagerpc.OK) {
			return true
		}
		if len(replyL.Value) != 1 || replyL.Value[0] != "value" {
			LOGE.Println("FAIL: GetList got wrong value")
			failCount++
			return true
		}
		if !isTimeOK(time.Since(ts)) {
			LOGE.Println("FAIL: concurrent GetList got blocked")
			failCount++
			return true
		}
		return false
	}

	if delayedRevokeList(key1, f) {
		return
	}
	fmt.Println("PASS")
	passCount++
}

// when revoking leases for key "x",
// storage server should stop leasing for "x"
// before revoking completes or old lease expires.
// this function tests the former case
func testDelayedRevokeListWithLeaseRequest1() {
	st.SetDelay(0.5) // Revoke finishes before lease expires
	defer st.ResetDelay()

	key1 := "revokelistkey:5"

	// function called during revoke of key1
	f := func() bool {
		ts := time.Now()
		// get key1 and want a lease
		replyL, err := st.GetList(key1, true)
		if checkErrorStatus(err, replyL.Status, storagerpc.OK) {
			return true
		}
		if isTimeOK(time.Since(ts)) {
			// in this case, server should reply old value and refuse lease
			if replyL.Lease.Granted || len(replyL.Value) != 1 || replyL.Value[0] != "old-value" {
				LOGE.Println("FAIL: server should return old value and not grant lease")
				failCount++
				return true
			}
		} else {
			if checkList(replyL.Value, []string{"old-value", "new-value"}) {
				return true
			}
			if !st.compRevoke[key1] || !replyL.Lease.Granted {
				LOGE.Println("FAIL: server should grant lease in this case")
				failCount++
				return true
			}
		}
		return false
	}

	if delayedRevokeList(key1, f) {
		return
	}
	fmt.Println("PASS")
	passCount++
}

// when revoking leases for key "x",
// storage server should stop leasing for "x"
// before revoking completes or old lease expires.
// this function tests the latter case
// The diff from the previous test is
// st.compRevoke[key1] in the else case
func testDelayedRevokeListWithLeaseRequest2() {
	st.SetDelay(2) // Lease expires before revoking finishes
	defer st.ResetDelay()

	key1 := "revokelistkey:15"

	// function called during revoke of key1
	f := func() bool {
		ts := time.Now()
		// get key1 and want a lease
		replyL, err := st.GetList(key1, true)
		if checkErrorStatus(err, replyL.Status, storagerpc.OK) {
			return true
		}
		if isTimeOK(time.Since(ts)) {
			// in this case, server should reply old value and refuse lease
			if replyL.Lease.Granted || len(replyL.Value) != 1 || replyL.Value[0] != "old-value" {
				LOGE.Println("FAIL: server should return old value and not grant lease")
				failCount++
				return true
			}
		} else {
			if checkList(replyL.Value, []string{"old-value", "new-value"}) {
				return true
			}
			if st.compRevoke[key1] || !replyL.Lease.Granted {
				LOGE.Println("FAIL: server should grant lease in this case")
				failCount++
				return true
			}
		}
		return false
	}

	if delayedRevokeList(key1, f) {
		return
	}
	fmt.Println("PASS")
	passCount++
}

// when revoking leases for key "x",
// storage server should hold upcoming updates for "x",
// until either all revocations complete or the lease expires
// this function tests the former case
func testDelayedRevokeListWithUpdate1() {
	st.SetDelay(0.5) // revocation takes longer, but still completes before lease expires
	defer st.ResetDelay()

	key1 := "revokelistkey:6"

	// function called during revoke of key1
	f := func() bool {
		// put key1, this should block
		replyP, err := st.AppendToList(key1, "newnew-value")
		if checkErrorStatus(err, replyP.Status, storagerpc.OK) {
			return true
		}
		if !st.compRevoke[key1] {
			LOGE.Println("FAIL: storage server should hold modification to key x during finishing revocating all lease holders of x")
			failCount++
			return true
		}
		replyL, err := st.GetList(key1, false)
		if checkErrorStatus(err, replyL.Status, storagerpc.OK) {
			return true
		}
		if checkList(replyL.Value, []string{"old-value", "new-value", "newnew-value"}) {
			return true
		}
		return false
	}

	if delayedRevokeList(key1, f) {
		return
	}
	fmt.Println("PASS")
	passCount++
}

// when revoking leases for key "x",
// storage server should hold upcoming updates for "x",
// until either all revocations complete or the lease expires
// this function tests the latter case
func testDelayedRevokeListWithUpdate2() {
	st.SetDelay(2) // lease expires before revocation completes
	defer st.ResetDelay()

	key1 := "revokelistkey:7"

	// function called during revoke of key1
	f := func() bool {
		ts := time.Now()
		// put key1, this should block
		replyP, err := st.AppendToList(key1, "newnew-value")
		if checkErrorStatus(err, replyP.Status, storagerpc.OK) {
			return true
		}
		d := time.Since(ts)
		if d < (storagerpc.LeaseSeconds+storagerpc.LeaseGuardSeconds-1)*time.Second {
			LOGE.Println("FAIL: storage server should hold this Put until leases expires key1")
			failCount++
			return true
		}
		if st.compRevoke[key1] {
			LOGE.Println("FAIL: storage server should not block this Put till the lease revoke of key1")
			failCount++
			return true
		}
		replyL, err := st.GetList(key1, false)
		if checkErrorStatus(err, replyL.Status, storagerpc.OK) {
			return true
		}
		if checkList(replyL.Value, []string{"old-value", "new-value", "newnew-value"}) {
			return true
		}
		return false
	}

	if delayedRevokeList(key1, f) {
		return
	}
	fmt.Println("PASS")
	passCount++
}

// remote libstores may not even reply all RevokeLease RPC calls.
// in this case, service should continue after lease expires
func testDelayedRevokeListWithUpdate3() {
	st.SetDelay(2) // lease expires before revocation completes
	defer st.ResetDelay()

	key1 := "revokelistkey:8"

	// function called during revoke of key1
	f := func() bool {
		// sleep here until lease expires on the remote server
		time.Sleep((storagerpc.LeaseSeconds + storagerpc.LeaseGuardSeconds) * time.Second)

		// put key1, this should not block
		ts := time.Now()
		replyP, err := st.AppendToList(key1, "newnew-value")
		if checkErrorStatus(err, replyP.Status, storagerpc.OK) {
			return true
		}
		if !isTimeOK(time.Since(ts)) {
			LOGE.Println("FAIL: storage server should not block this Put")
			failCount++
			return true
		}
		// get key1 and want lease, this should not block
		ts = time.Now()
		replyL, err := st.GetList(key1, true)
		if checkErrorStatus(err, replyL.Status, storagerpc.OK) {
			return true
		}
		if checkList(replyL.Value, []string{"old-value", "new-value", "newnew-value"}) {
			return true
		}
		if !isTimeOK(time.Since(ts)) {
			LOGE.Println("FAIL: storage server should not block this Get")
			failCount++
			return true
		}
		return false
	}

	if delayedRevokeList(key1, f) {
		return
	}
	fmt.Println("PASS")
	passCount++
}

func main() {
	jtests := []testFunc{{"testInitStorageServers", testInitStorageServers}}
	btests := []testFunc{
		{"testPutGet", testPutGet},
		{"testAppendGetRemoveList", testAppendGetRemoveList},
		{"testUpdateWithoutLease", testUpdateWithoutLease},
		{"testUpdateBeforeLeaseExpire", testUpdateBeforeLeaseExpire},
		{"testUpdateAfterLeaseExpire", testUpdateAfterLeaseExpire},
		{"testDelayedRevokeWithoutBlocking", testDelayedRevokeWithoutBlocking},
		{"testDelayedRevokeWithLeaseRequest1", testDelayedRevokeWithLeaseRequest1},
		{"testDelayedRevokeWithLeaseRequest2", testDelayedRevokeWithLeaseRequest2},
		{"testDelayedRevokeWithUpdate1", testDelayedRevokeWithUpdate1},
		{"testDelayedRevokeWithUpdate2", testDelayedRevokeWithUpdate2},
		{"testDelayedRevokeWithUpdate3", testDelayedRevokeWithUpdate3},
		{"testUpdateListWithoutLease", testUpdateListWithoutLease},
		{"testUpdateListBeforeLeaseExpire", testUpdateListBeforeLeaseExpire},
		{"testUpdateListAfterLeaseExpire", testUpdateListAfterLeaseExpire},
		{"testDelayedRevokeListWithoutBlocking", testDelayedRevokeListWithoutBlocking},
		{"testDelayedRevokeListWithLeaseRequest1", testDelayedRevokeListWithLeaseRequest1},
		{"testDelayedRevokeListWithLeaseRequest2", testDelayedRevokeListWithLeaseRequest2},
		{"testDelayedRevokeListWithUpdate1", testDelayedRevokeListWithUpdate1},
		{"testDelayedRevokeListWithUpdate2", testDelayedRevokeListWithUpdate2},
		{"testDelayedRevokeListWithUpdate3", testDelayedRevokeListWithUpdate3},
	}

	flag.Parse()
	if flag.NArg() < 1 {
		LOGE.Fatalln("Usage: storagetest <storage master>")
	}

	// Run the tests with a single tester
	storageTester, err := initStorageTester(flag.Arg(0), fmt.Sprintf("localhost:%d", *portnum))
	if err != nil {
		LOGE.Fatalln("Failed to initialize test:", err)
	}
	st = storageTester

	switch *testType {
	case 1:
		for _, t := range jtests {
			if b, err := regexp.MatchString(*testRegex, t.name); b && err == nil {
				fmt.Printf("Running %s:\n", t.name)
				t.f()
			}
		}
	case 2:
		for _, t := range btests {
			if b, err := regexp.MatchString(*testRegex, t.name); b && err == nil {
				fmt.Printf("Running %s:\n", t.name)
				t.f()
			}
		}
	}

	fmt.Printf("Passed (%d/%d) tests\n", passCount, passCount+failCount)
}
