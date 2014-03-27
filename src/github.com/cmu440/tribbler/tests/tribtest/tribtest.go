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
	"strconv"
	"strings"

	"github.com/cmu440/tribbler/rpc/storagerpc"
	"github.com/cmu440/tribbler/rpc/tribrpc"
	"github.com/cmu440/tribbler/tests/proxycounter"
	"github.com/cmu440/tribbler/tribserver"
)

type testFunc struct {
	name string
	f    func()
}

var (
	port      = flag.Int("port", 9010, "TribServer port number")
	testRegex = flag.String("t", "", "test to run")
	passCount int
	failCount int
	pc        proxycounter.ProxyCounter
	ts        tribserver.TribServer
)

var statusMap = map[tribrpc.Status]string{
	tribrpc.OK:               "OK",
	tribrpc.NoSuchUser:       "NoSuchUser",
	tribrpc.NoSuchTargetUser: "NoSuchTargetUser",
	tribrpc.Exists:           "Exists",
	0:                        "Unknown",
}

var LOGE = log.New(os.Stderr, "", log.Lshortfile|log.Lmicroseconds)

func initTribServer(masterServerHostPort string, tribServerPort int) error {
	tribServerHostPort := net.JoinHostPort("localhost", strconv.Itoa(tribServerPort))
	proxyCounter, err := proxycounter.NewProxyCounter(masterServerHostPort, tribServerHostPort)
	if err != nil {
		LOGE.Println("Failed to setup test:", err)
		return err
	}
	pc = proxyCounter
	rpc.RegisterName("StorageServer", storagerpc.Wrap(pc))

	// Create and start the TribServer.
	tribServer, err := tribserver.NewTribServer(masterServerHostPort, tribServerHostPort)
	if err != nil {
		LOGE.Println("Failed to create TribServer:", err)
		return err
	}
	ts = tribServer
	return nil
}

// Cleanup tribserver and rpc hooks
func cleanupTribServer(l net.Listener) {
	// Close listener to stop http serve thread
	if l != nil {
		l.Close()
	}
	// Recreate default http serve mux
	http.DefaultServeMux = http.NewServeMux()
	// Recreate default rpc server
	rpc.DefaultServer = rpc.NewServer()
	// Unset tribserver just in case
	ts = nil
}

// Check rpc and byte count limits.
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

// Check error and status
func checkErrorStatus(err error, status, expectedStatus tribrpc.Status) bool {
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

// Check subscriptions
func checkSubscriptions(subs, expectedSubs []string) bool {
	if len(subs) != len(expectedSubs) {
		LOGE.Printf("FAIL: incorrect subscriptions %v, expected subscriptions %v\n", subs, expectedSubs)
		failCount++
		return true
	}
	m := make(map[string]bool)
	for _, s := range subs {
		m[s] = true
	}
	for _, s := range expectedSubs {
		if m[s] == false {
			LOGE.Printf("FAIL: incorrect subscriptions %v, expected subscriptions %v\n", subs, expectedSubs)
			failCount++
			return true
		}
	}
	return false
}

// Check tribbles
func checkTribbles(tribbles, expectedTribbles []tribrpc.Tribble) bool {
	if len(tribbles) != len(expectedTribbles) {
		LOGE.Printf("FAIL: incorrect tribbles %v, expected tribbles %v\n", tribbles, expectedTribbles)
		failCount++
		return true
	}
	lastTime := int64(0)
	for i := len(tribbles) - 1; i >= 0; i-- {
		if tribbles[i].UserID != expectedTribbles[i].UserID {
			LOGE.Printf("FAIL: incorrect tribbles %v, expected tribbles %v\n", tribbles, expectedTribbles)
			failCount++
			return true
		}
		if tribbles[i].Contents != expectedTribbles[i].Contents {
			LOGE.Printf("FAIL: incorrect tribbles %v, expected tribbles %v\n", tribbles, expectedTribbles)
			failCount++
			return true
		}
		if tribbles[i].Posted.UnixNano() < lastTime {
			LOGE.Println("FAIL: tribble timestamps not in reverse chronological order")
			failCount++
			return true
		}
		lastTime = tribbles[i].Posted.UnixNano()
	}
	return false
}

// Helper functions
func createUser(user string) (error, tribrpc.Status) {
	args := &tribrpc.CreateUserArgs{UserID: user}
	var reply tribrpc.CreateUserReply
	err := ts.CreateUser(args, &reply)
	return err, reply.Status
}

func addSubscription(user, target string) (error, tribrpc.Status) {
	args := &tribrpc.SubscriptionArgs{UserID: user, TargetUserID: target}
	var reply tribrpc.SubscriptionReply
	err := ts.AddSubscription(args, &reply)
	return err, reply.Status
}

func removeSubscription(user, target string) (error, tribrpc.Status) {
	args := &tribrpc.SubscriptionArgs{UserID: user, TargetUserID: target}
	var reply tribrpc.SubscriptionReply
	err := ts.RemoveSubscription(args, &reply)
	return err, reply.Status
}

func getSubscription(user string) (error, tribrpc.Status, []string) {
	args := &tribrpc.GetSubscriptionsArgs{UserID: user}
	var reply tribrpc.GetSubscriptionsReply
	err := ts.GetSubscriptions(args, &reply)
	return err, reply.Status, reply.UserIDs
}

func postTribble(user, contents string) (error, tribrpc.Status) {
	args := &tribrpc.PostTribbleArgs{UserID: user, Contents: contents}
	var reply tribrpc.PostTribbleReply
	err := ts.PostTribble(args, &reply)
	return err, reply.Status
}

func getTribbles(user string) (error, tribrpc.Status, []tribrpc.Tribble) {
	args := &tribrpc.GetTribblesArgs{UserID: user}
	var reply tribrpc.GetTribblesReply
	err := ts.GetTribbles(args, &reply)
	return err, reply.Status, reply.Tribbles
}

func getTribblesBySubscription(user string) (error, tribrpc.Status, []tribrpc.Tribble) {
	args := &tribrpc.GetTribblesArgs{UserID: user}
	var reply tribrpc.GetTribblesReply
	err := ts.GetTribblesBySubscription(args, &reply)
	return err, reply.Status, reply.Tribbles
}

// Create valid user
func testCreateUserValid() {
	pc.Reset()
	err, status := createUser("user")
	if checkErrorStatus(err, status, tribrpc.OK) {
		return
	}
	if checkLimits(10, 1000) {
		return
	}
	fmt.Println("PASS")
	passCount++
}

// Create duplicate user
func testCreateUserDuplicate() {
	createUser("user")
	pc.Reset()
	err, status := createUser("user")
	if checkErrorStatus(err, status, tribrpc.Exists) {
		return
	}
	if checkLimits(10, 1000) {
		return
	}
	fmt.Println("PASS")
	passCount++
}

// Add subscription with invalid user
func testAddSubscriptionInvalidUser() {
	createUser("user")
	pc.Reset()
	err, status := addSubscription("invalidUser", "user")
	if checkErrorStatus(err, status, tribrpc.NoSuchUser) {
		return
	}
	if checkLimits(10, 1000) {
		return
	}
	fmt.Println("PASS")
	passCount++
}

// Add subscription with invaild target user
func testAddSubscriptionInvalidTargetUser() {
	createUser("user")
	pc.Reset()
	err, status := addSubscription("user", "invalidUser")
	if checkErrorStatus(err, status, tribrpc.NoSuchTargetUser) {
		return
	}
	if checkLimits(10, 1000) {
		return
	}
	fmt.Println("PASS")
	passCount++
}

// Add valid subscription
func testAddSubscriptionValid() {
	createUser("user1")
	createUser("user2")
	pc.Reset()
	err, status := addSubscription("user1", "user2")
	if checkErrorStatus(err, status, tribrpc.OK) {
		return
	}
	if checkLimits(10, 1000) {
		return
	}
	fmt.Println("PASS")
	passCount++
}

// Add duplicate subscription
func testAddSubscriptionDuplicate() {
	createUser("user1")
	createUser("user2")
	addSubscription("user1", "user2")
	pc.Reset()
	err, status := addSubscription("user1", "user2")
	if checkErrorStatus(err, status, tribrpc.Exists) {
		return
	}
	if checkLimits(10, 1000) {
		return
	}
	fmt.Println("PASS")
	passCount++
}

// Remove subscription with invalid user
func testRemoveSubscriptionInvalidUser() {
	createUser("user")
	pc.Reset()
	err, status := removeSubscription("invalidUser", "user")
	if checkErrorStatus(err, status, tribrpc.NoSuchUser) {
		return
	}
	if checkLimits(10, 1000) {
		return
	}
	fmt.Println("PASS")
	passCount++
}

// Remove valid subscription
func testRemoveSubscriptionValid() {
	createUser("user1")
	createUser("user2")
	addSubscription("user1", "user2")
	pc.Reset()
	err, status := removeSubscription("user1", "user2")
	if checkErrorStatus(err, status, tribrpc.OK) {
		return
	}
	if checkLimits(10, 1000) {
		return
	}
	fmt.Println("PASS")
	passCount++
}

// Remove subscription with missing target user
func testRemoveSubscriptionMissingTarget() {
	createUser("user1")
	createUser("user2")
	removeSubscription("user1", "user2")
	pc.Reset()
	err, status := removeSubscription("user1", "user2")
	if checkErrorStatus(err, status, tribrpc.NoSuchTargetUser) {
		return
	}
	if checkLimits(10, 1000) {
		return
	}
	fmt.Println("PASS")
	passCount++
}

// Get subscription with invalid user
func testGetSubscriptionInvalidUser() {
	pc.Reset()
	err, status, _ := getSubscription("invalidUser")
	if checkErrorStatus(err, status, tribrpc.NoSuchUser) {
		return
	}
	if checkLimits(10, 1000) {
		return
	}
	fmt.Println("PASS")
	passCount++
}

// Get valid subscription
func testGetSubscriptionValid() {
	createUser("user1")
	createUser("user2")
	createUser("user3")
	createUser("user4")
	addSubscription("user1", "user2")
	addSubscription("user1", "user3")
	addSubscription("user1", "user4")
	pc.Reset()
	err, status, subs := getSubscription("user1")
	if checkErrorStatus(err, status, tribrpc.OK) {
		return
	}
	if checkSubscriptions(subs, []string{"user2", "user3", "user4"}) {
		return
	}
	if checkLimits(10, 1000) {
		return
	}
	fmt.Println("PASS")
	passCount++
}

// Post tribble with invalid user
func testPostTribbleInvalidUser() {
	pc.Reset()
	err, status := postTribble("invalidUser", "contents")
	if checkErrorStatus(err, status, tribrpc.NoSuchUser) {
		return
	}
	if checkLimits(10, 1000) {
		return
	}
	fmt.Println("PASS")
	passCount++
}

// Post valid tribble
func testPostTribbleValid() {
	createUser("user")
	pc.Reset()
	err, status := postTribble("user", "contents")
	if checkErrorStatus(err, status, tribrpc.OK) {
		return
	}
	if checkLimits(10, 1000) {
		return
	}
	fmt.Println("PASS")
	passCount++
}

// Get tribbles invalid user
func testGetTribblesInvalidUser() {
	pc.Reset()
	err, status, _ := getTribbles("invalidUser")
	if checkErrorStatus(err, status, tribrpc.NoSuchUser) {
		return
	}
	if checkLimits(10, 1000) {
		return
	}
	fmt.Println("PASS")
	passCount++
}

// Get tribbles 0 tribbles
func testGetTribblesZeroTribbles() {
	createUser("tribUser")
	pc.Reset()
	err, status, tribbles := getTribbles("tribUser")
	if checkErrorStatus(err, status, tribrpc.OK) {
		return
	}
	if checkTribbles(tribbles, []tribrpc.Tribble{}) {
		return
	}
	if checkLimits(10, 1000) {
		return
	}
	fmt.Println("PASS")
	passCount++
}

// Get tribbles < 100 tribbles
func testGetTribblesFewTribbles() {
	createUser("tribUser")
	expectedTribbles := []tribrpc.Tribble{}
	for i := 0; i < 5; i++ {
		expectedTribbles = append(expectedTribbles, tribrpc.Tribble{UserID: "tribUser", Contents: fmt.Sprintf("contents%d", i)})
	}
	for i := len(expectedTribbles) - 1; i >= 0; i-- {
		postTribble(expectedTribbles[i].UserID, expectedTribbles[i].Contents)
	}
	pc.Reset()
	err, status, tribbles := getTribbles("tribUser")
	if checkErrorStatus(err, status, tribrpc.OK) {
		return
	}
	if checkTribbles(tribbles, expectedTribbles) {
		return
	}
	if checkLimits(50, 5000) {
		return
	}
	fmt.Println("PASS")
	passCount++
}

// Get tribbles > 100 tribbles
func testGetTribblesManyTribbles() {
	createUser("tribUser")
	postTribble("tribUser", "should not see this old msg")
	expectedTribbles := []tribrpc.Tribble{}
	for i := 0; i < 100; i++ {
		expectedTribbles = append(expectedTribbles, tribrpc.Tribble{UserID: "tribUser", Contents: fmt.Sprintf("contents%d", i)})
	}
	for i := len(expectedTribbles) - 1; i >= 0; i-- {
		postTribble(expectedTribbles[i].UserID, expectedTribbles[i].Contents)
	}
	pc.Reset()
	err, status, tribbles := getTribbles("tribUser")
	if checkErrorStatus(err, status, tribrpc.OK) {
		return
	}
	if checkTribbles(tribbles, expectedTribbles) {
		return
	}
	if checkLimits(200, 30000) {
		return
	}
	fmt.Println("PASS")
	passCount++
}

// Get tribbles by subscription invalid user
func testGetTribblesBySubscriptionInvalidUser() {
	pc.Reset()
	err, status, _ := getTribblesBySubscription("invalidUser")
	if checkErrorStatus(err, status, tribrpc.NoSuchUser) {
		return
	}
	if checkLimits(10, 1000) {
		return
	}
	fmt.Println("PASS")
	passCount++
}

// Get tribbles by subscription no subscriptions
func testGetTribblesBySubscriptionNoSubscriptions() {
	createUser("tribUser")
	postTribble("tribUser", "contents")
	pc.Reset()
	err, status, tribbles := getTribblesBySubscription("tribUser")
	if checkErrorStatus(err, status, tribrpc.OK) {
		return
	}
	if checkTribbles(tribbles, []tribrpc.Tribble{}) {
		return
	}
	if checkLimits(10, 1000) {
		return
	}
	fmt.Println("PASS")
	passCount++
}

// Get tribbles by subscription 0 tribbles
func testGetTribblesBySubscriptionZeroTribbles() {
	createUser("tribUser1")
	createUser("tribUser2")
	addSubscription("tribUser1", "tribUser2")
	pc.Reset()
	err, status, tribbles := getTribbles("tribUser1")
	if checkErrorStatus(err, status, tribrpc.OK) {
		return
	}
	if checkTribbles(tribbles, []tribrpc.Tribble{}) {
		return
	}
	if checkLimits(10, 1000) {
		return
	}
	fmt.Println("PASS")
	passCount++
}

// Get tribbles by subscription < 100 tribbles
func testGetTribblesBySubscriptionFewTribbles() {
	createUser("tribUser1")
	createUser("tribUser2")
	createUser("tribUser3")
	createUser("tribUser4")
	addSubscription("tribUser1", "tribUser2")
	addSubscription("tribUser1", "tribUser3")
	addSubscription("tribUser1", "tribUser4")
	postTribble("tribUser1", "should not see this unsubscribed msg")
	expectedTribbles := []tribrpc.Tribble{tribrpc.Tribble{UserID: "tribUser2", Contents: "contents"}, tribrpc.Tribble{UserID: "tribUser4", Contents: "contents"}}
	for i := len(expectedTribbles) - 1; i >= 0; i-- {
		postTribble(expectedTribbles[i].UserID, expectedTribbles[i].Contents)
	}
	pc.Reset()
	err, status, tribbles := getTribblesBySubscription("tribUser1")
	if checkErrorStatus(err, status, tribrpc.OK) {
		return
	}
	if checkTribbles(tribbles, expectedTribbles) {
		return
	}
	if checkLimits(20, 2000) {
		return
	}
	fmt.Println("PASS")
	passCount++
}

// Get tribbles by subscription > 100 tribbles
func testGetTribblesBySubscriptionManyTribbles() {
	createUser("tribUser1")
	createUser("tribUser2")
	createUser("tribUser3")
	createUser("tribUser4")
	addSubscription("tribUser1", "tribUser2")
	addSubscription("tribUser1", "tribUser3")
	addSubscription("tribUser1", "tribUser4")
	postTribble("tribUser1", "should not see this old msg")
	postTribble("tribUser2", "should not see this old msg")
	postTribble("tribUser3", "should not see this old msg")
	postTribble("tribUser4", "should not see this old msg")
	expectedTribbles := []tribrpc.Tribble{}
	for i := 0; i < 100; i++ {
		expectedTribbles = append(expectedTribbles, tribrpc.Tribble{UserID: fmt.Sprintf("tribUser%d", (i%3)+2), Contents: fmt.Sprintf("contents%d", i)})
	}
	for i := len(expectedTribbles) - 1; i >= 0; i-- {
		postTribble(expectedTribbles[i].UserID, expectedTribbles[i].Contents)
	}
	pc.Reset()
	err, status, tribbles := getTribblesBySubscription("tribUser1")
	if checkErrorStatus(err, status, tribrpc.OK) {
		return
	}
	if checkTribbles(tribbles, expectedTribbles) {
		return
	}
	if checkLimits(200, 30000) {
		return
	}
	fmt.Println("PASS")
	passCount++
}

// Get tribbles by subscription all recent tribbles by one subscription
func testGetTribblesBySubscriptionManyTribbles2() {
	createUser("tribUser1b")
	createUser("tribUser2b")
	createUser("tribUser3b")
	createUser("tribUser4b")
	addSubscription("tribUser1b", "tribUser2b")
	addSubscription("tribUser1b", "tribUser3b")
	addSubscription("tribUser1b", "tribUser4b")
	postTribble("tribUser1b", "should not see this old msg")
	postTribble("tribUser2b", "should not see this old msg")
	postTribble("tribUser3b", "should not see this old msg")
	postTribble("tribUser4b", "should not see this old msg")
	expectedTribbles := []tribrpc.Tribble{}
	for i := 0; i < 100; i++ {
		expectedTribbles = append(expectedTribbles, tribrpc.Tribble{UserID: fmt.Sprintf("tribUser3b"), Contents: fmt.Sprintf("contents%d", i)})
	}
	for i := len(expectedTribbles) - 1; i >= 0; i-- {
		postTribble(expectedTribbles[i].UserID, expectedTribbles[i].Contents)
	}
	pc.Reset()
	err, status, tribbles := getTribblesBySubscription("tribUser1b")
	if checkErrorStatus(err, status, tribrpc.OK) {
		return
	}
	if checkTribbles(tribbles, expectedTribbles) {
		return
	}
	if checkLimits(200, 30000) {
		return
	}
	fmt.Println("PASS")
	passCount++
}

// Get tribbles by subscription test not performing too many RPCs or transferring too much data
func testGetTribblesBySubscriptionManyTribbles3() {
	createUser("tribUser1c")
	createUser("tribUser2c")
	createUser("tribUser3c")
	createUser("tribUser4c")
	createUser("tribUser5c")
	createUser("tribUser6c")
	createUser("tribUser7c")
	createUser("tribUser8c")
	createUser("tribUser9c")
	addSubscription("tribUser1c", "tribUser2c")
	addSubscription("tribUser1c", "tribUser3c")
	addSubscription("tribUser1c", "tribUser4c")
	addSubscription("tribUser1c", "tribUser5c")
	addSubscription("tribUser1c", "tribUser6c")
	addSubscription("tribUser1c", "tribUser7c")
	addSubscription("tribUser1c", "tribUser8c")
	addSubscription("tribUser1c", "tribUser9c")
	postTribble("tribUser1c", "should not see this old msg")
	postTribble("tribUser2c", "should not see this old msg")
	postTribble("tribUser3c", "should not see this old msg")
	postTribble("tribUser4c", "should not see this old msg")
	postTribble("tribUser5c", "should not see this old msg")
	postTribble("tribUser6c", "should not see this old msg")
	postTribble("tribUser7c", "should not see this old msg")
	postTribble("tribUser8c", "should not see this old msg")
	postTribble("tribUser9c", "should not see this old msg")
	longContents := strings.Repeat("this sentence is 30 char long\n", 30)
	for i := 0; i < 100; i++ {
		for j := 1; j <= 9; j++ {
			postTribble(fmt.Sprintf("tribUser%dc", j), longContents)
		}
	}
	expectedTribbles := []tribrpc.Tribble{}
	for i := 0; i < 100; i++ {
		expectedTribbles = append(expectedTribbles, tribrpc.Tribble{UserID: fmt.Sprintf("tribUser%dc", (i%8)+2), Contents: fmt.Sprintf("contents%d", i)})
	}
	for i := len(expectedTribbles) - 1; i >= 0; i-- {
		postTribble(expectedTribbles[i].UserID, expectedTribbles[i].Contents)
	}
	pc.Reset()
	err, status, tribbles := getTribblesBySubscription("tribUser1c")
	if checkErrorStatus(err, status, tribrpc.OK) {
		return
	}
	if checkTribbles(tribbles, expectedTribbles) {
		return
	}
	if checkLimits(200, 200000) {
		return
	}
	fmt.Println("PASS")
	passCount++
}

func main() {
	tests := []testFunc{
		{"testCreateUserValid", testCreateUserValid},
		{"testCreateUserDuplicate", testCreateUserDuplicate},
		{"testAddSubscriptionInvalidUser", testAddSubscriptionInvalidUser},
		{"testAddSubscriptionInvalidTargetUser", testAddSubscriptionInvalidTargetUser},
		{"testAddSubscriptionValid", testAddSubscriptionValid},
		{"testAddSubscriptionDuplicate", testAddSubscriptionDuplicate},
		{"testRemoveSubscriptionInvalidUser", testRemoveSubscriptionInvalidUser},
		{"testRemoveSubscriptionValid", testRemoveSubscriptionValid},
		{"testRemoveSubscriptionMissingTarget", testRemoveSubscriptionMissingTarget},
		{"testGetSubscriptionInvalidUser", testGetSubscriptionInvalidUser},
		{"testGetSubscriptionValid", testGetSubscriptionValid},
		{"testPostTribbleInvalidUser", testPostTribbleInvalidUser},
		{"testPostTribbleValid", testPostTribbleValid},
		{"testGetTribblesInvalidUser", testGetTribblesInvalidUser},
		{"testGetTribblesZeroTribbles", testGetTribblesZeroTribbles},
		{"testGetTribblesFewTribbles", testGetTribblesFewTribbles},
		{"testGetTribblesManyTribbles", testGetTribblesManyTribbles},
		{"testGetTribblesBySubscriptionInvalidUser", testGetTribblesBySubscriptionInvalidUser},
		{"testGetTribblesBySubscriptionNoSubscriptions", testGetTribblesBySubscriptionNoSubscriptions},
		{"testGetTribblesBySubscriptionZeroTribbles", testGetTribblesBySubscriptionZeroTribbles},
		{"testGetTribblesBySubscriptionZeroTribbles", testGetTribblesBySubscriptionZeroTribbles},
		{"testGetTribblesBySubscriptionFewTribbles", testGetTribblesBySubscriptionFewTribbles},
		{"testGetTribblesBySubscriptionManyTribbles", testGetTribblesBySubscriptionManyTribbles},
		{"testGetTribblesBySubscriptionManyTribbles2", testGetTribblesBySubscriptionManyTribbles2},
		{"testGetTribblesBySubscriptionManyTribbles3", testGetTribblesBySubscriptionManyTribbles3},
	}

	flag.Parse()
	if flag.NArg() < 1 {
		LOGE.Fatal("Usage: tribtest <storage master host:port>")
	}

	if err := initTribServer(flag.Arg(0), *port); err != nil {
		LOGE.Fatalln("Failed to setup TribServer:", err)
	}

	// Run tests.
	for _, t := range tests {
		if b, err := regexp.MatchString(*testRegex, t.name); b && err == nil {
			fmt.Printf("Running %s:\n", t.name)
			t.f()
		}
	}

	fmt.Printf("Passed (%d/%d) tests\n", passCount, passCount+failCount)
}
