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

func main() {
	flag.Parse()
	if flag.NArg() < 2 {
		LOGE.Fatalln("Usage: ./stressclient <user> <numTargets>")
	}

	client, err := tribclient.NewTribClient("localhost", *portnum)
	if err != nil {
		LOGE.Fatalf("FAIL: NewTribClient returned error '%s'\n", err)
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

	status, err := client.CreateUser(user)
	if err != nil {
		LOGE.Fatalf("FAIL: error when creating user %s\n", user)
	}
	if status != tribrpc.OK {
		LOGE.Fatalln("FAIL: CreateUser returned error status", statusMap[status])
	}

	tribIndex := 0
	if *seed == 0 {
		rand.Seed(time.Now().UnixNano())
	} else {
		rand.Seed(*seed)
	}

	for i := 0; i < *numCmds; i++ {
		funcnum := rand.Intn(6)

		switch funcnum {
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
