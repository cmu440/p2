package main

import (
	"flag"
	"fmt"
	"log"
	"math/rand"
	"os"
	"strconv"
	"strings"
	"time"

	"github.com/cmu440/tribbler/rpc/tribrpc"
	"github.com/cmu440/tribbler/tribclient"
)

const (
	GetSubscription = iota
	AddSubscription
	RemoveSubscription
	GetTribbles
	PostTribble
	GetTribblesBySubscription
)

var (
	portnum  = flag.Int("port", 9010, "server port # to connect to")
	clientId = flag.String("clientId", "0", "client id for user")
	numCmds  = flag.Int("numCmds", 1000, "number of random commands to execute")
	seed     = flag.Int64("seed", 0, "seed for random number generator used to execute commands")
)

var LOGE = log.New(os.Stderr, "", log.Lshortfile|log.Lmicroseconds)

var statusMap = map[tribrpc.Status]string{
	tribrpc.OK:               "OK",
	tribrpc.NoSuchUser:       "NoSuchUser",
	tribrpc.NoSuchTargetUser: "NoSuchTargetUser",
	tribrpc.Exists:           "Exists",
	0:                        "Unknown",
}

var (
	// Debugging information (counts the total number of operations performed).
	gs, as, rs, gt, pt, gtbs int
	// Set this to true to print debug information.
	debug bool
)

func main() {
	flag.Parse()
	if flag.NArg() < 2 {
		LOGE.Fatalln("Usage: ./stressclient <user> <numTargets>")
	}

	client, err := tribclient.NewTribClient("localhost", *portnum)
	if err != nil {
		LOGE.Fatalln("FAIL: NewTribClient returned error:", err)
	}

	user := flag.Arg(0)
	userNum, err := strconv.Atoi(user)
	if err != nil {
		LOGE.Fatalf("FAIL: user %s not an integer\n", user)
	}
	numTargets, err := strconv.Atoi(flag.Arg(1))
	if err != nil {
		LOGE.Fatalf("FAIL: numTargets invalid %s\n", flag.Arg(1))
	}

	_, err = client.CreateUser(user)
	if err != nil {
		LOGE.Fatalf("FAIL: error when creating userID '%s': %s\n", user, err)
	}

	tribIndex := 0
	if *seed == 0 {
		rand.Seed(time.Now().UnixNano())
	} else {
		rand.Seed(*seed)
	}

	cmds := make([]int, *numCmds)
	for i := 0; i < *numCmds; i++ {
		cmds[i] = rand.Intn(6)
		switch cmds[i] {
		case GetSubscription:
			gs++
		case AddSubscription:
			as++
		case RemoveSubscription:
			rs++
		case GetTribbles:
			gt++
		case PostTribble:
			pt++
		case GetTribblesBySubscription:
			gtbs++
		}
	}

	if debug {
		// Prints out the total number of operations that will be performed.
		fmt.Println("GetSubscriptions:", gs)
		fmt.Println("AddSubscription:", as)
		fmt.Println("RemoveSubscription:", rs)
		fmt.Println("GetTribbles:", gt)
		fmt.Println("PostTribble:", pt)
		fmt.Println("GetTribblesBySubscription:", gtbs)
	}

	for _, cmd := range cmds {
		switch cmd {
		case GetSubscription:
			subscriptions, status, err := client.GetSubscriptions(user)
			if err != nil {
				LOGE.Fatalf("FAIL: GetSubscriptions returned error '%s'\n", err)
			}
			if status == 0 || status == tribrpc.NoSuchUser {
				LOGE.Fatalf("FAIL: GetSubscriptions returned error status '%s'\n", statusMap[status])
			}
			if !validateSubscriptions(&subscriptions) {
				LOGE.Fatalln("FAIL: failed while validating returned subscriptions")
			}
		case AddSubscription:
			target := rand.Intn(numTargets)
			status, err := client.AddSubscription(user, strconv.Itoa(target))
			if err != nil {
				LOGE.Fatalf("FAIL: AddSubscription returned error '%s'\n", err)
			}
			if status == 0 || status == tribrpc.NoSuchUser {
				LOGE.Fatalf("FAIL: AddSubscription returned error status '%s'\n", statusMap[status])
			}
		case RemoveSubscription:
			target := rand.Intn(numTargets)
			status, err := client.RemoveSubscription(user, strconv.Itoa(target))
			if err != nil {
				LOGE.Fatalf("FAIL: RemoveSubscription returned error '%s'\n", err)
			}
			if status == 0 || status == tribrpc.NoSuchUser {
				LOGE.Fatalf("FAIL: RemoveSubscription returned error status '%s'\n", statusMap[status])
			}
		case GetTribbles:
			target := rand.Intn(numTargets)
			tribbles, status, err := client.GetTribbles(strconv.Itoa(target))
			if err != nil {
				LOGE.Fatalf("FAIL: GetTribbles returned error '%s'\n", err)
			}
			if status == 0 {
				LOGE.Fatalf("FAIL: GetTribbles returned error status '%s'\n", statusMap[status])
			}
			if !validateTribbles(&tribbles, numTargets) {
				LOGE.Fatalln("FAIL: failed while validating returned tribbles")
			}
		case PostTribble:
			tribVal := userNum + tribIndex*numTargets
			msg := fmt.Sprintf("%d;%s", tribVal, *clientId)
			status, err := client.PostTribble(user, msg)
			if err != nil {
				LOGE.Fatalf("FAIL: PostTribble returned error '%s'\n", err)
			}
			if status == 0 || status == tribrpc.NoSuchUser {
				LOGE.Fatalf("FAIL: PostTribble returned error status '%s'\n", statusMap[status])
			}
			tribIndex++
		case GetTribblesBySubscription:
			tribbles, status, err := client.GetTribblesBySubscription(user)
			if err != nil {
				LOGE.Fatalf("FAIL: GetTribblesBySubscription returned error '%s'\n", err)
			}
			if status == 0 || status == tribrpc.NoSuchUser {
				LOGE.Fatalf("FAIL: GetTribblesBySubscription returned error status '%s'\n", statusMap[status])
			}
			if !validateTribbles(&tribbles, numTargets) {
				LOGE.Fatalln("FAIL: failed while validating returned tribbles")
			}
		}
	}
	fmt.Println("PASS")
	os.Exit(7)
}

func validateSubscriptions(subscriptions *[]string) bool {
	subscriptionSet := make(map[string]bool, len(*subscriptions))
	for _, subscription := range *subscriptions {
		if subscriptionSet[subscription] == true {
			return false
		}
		subscriptionSet[subscription] = true
	}
	return true
}

func validateTribbles(tribbles *[]tribrpc.Tribble, numTargets int) bool {
	userIdToLastVal := make(map[string]int, len(*tribbles))
	for _, tribble := range *tribbles {
		valAndId := strings.Split(tribble.Contents, ";")
		val, err := strconv.Atoi(valAndId[0])
		if err != nil {
			return false
		}
		user, err := strconv.Atoi(tribble.UserID)
		if err != nil {
			return false
		}
		userClientId := fmt.Sprintf("%s;%s", tribble.UserID, valAndId[1])
		lastVal := userIdToLastVal[userClientId]
		if val%numTargets == user && (lastVal == 0 || lastVal == val+numTargets) {
			userIdToLastVal[userClientId] = val
		} else {
			return false
		}
	}
	return true
}
