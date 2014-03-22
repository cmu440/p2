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

var (
	portnum  = flag.Int("port", 9010, "server port # to connect to")
	clientId = flag.String("clientId", "0", "client id for user")
	numCmds  = flag.Int("numCmds", 1000, "number of random commands to execute")
	seed     = flag.Int64("seed", 0, "seed for random number generator used to execute commands")
)

func init() {
	log.SetFlags(log.Lshortfile | log.Lmicroseconds)
}

func main() {
	flag.Parse()
	if flag.NArg() < 2 {
		log.Fatalln("Usage: ./stressclient <user> <numTargets>")
	}

	client, _ := tribclient.NewTribClient("localhost", *portnum)

	user := flag.Arg(0)
	userNum, err := strconv.Atoi(user)
	if err != nil {
		log.Fatalf("FAIL: user %s not an integer\n", user)
	}
	numTargets, err := strconv.Atoi(flag.Arg(1))
	if err != nil {
		log.Fatalf("FAIL: numTargets invalid %s\n", flag.Arg(1))
	}

	client.CreateUser(user)
	if err != nil {
		log.Fatalf("FAIL: error when creating user %s\n", user)
		return
	}

	failed := false
	tribIndex := 0
	if *seed == 0 {
		rand.Seed(time.Now().UnixNano())
	} else {
		rand.Seed(*seed)
	}
	for i := 0; i < *numCmds; i++ {
		funcnum := rand.Intn(6)

		switch funcnum {
		case 0: //client.GetSubscription
			subscriptions, status, err := client.GetSubscriptions(user)
			if err != nil || status == tribrpc.NoSuchUser {
				failTest("error with GetSubscriptions")
			}
			failed = !validateSubscriptions(&subscriptions)
		case 1: //client.AddSubscription
			target := rand.Intn(numTargets)
			status, err := client.AddSubscription(user, strconv.Itoa(target))
			if err != nil || status == tribrpc.NoSuchUser {
				failTest("error with AddSubscription")
			}
		case 2: //client.RemoveSubscription
			target := rand.Intn(numTargets)
			status, err := client.RemoveSubscription(user, strconv.Itoa(target))
			if err != nil || status == tribrpc.NoSuchUser {
				failTest("error with RemoveSubscription")
			}
		case 3: //client.GetTribbles
			target := rand.Intn(numTargets)
			tribbles, _, err := client.GetTribbles(strconv.Itoa(target))
			if err != nil {
				failTest("error with GetTribbles")
			}
			failed = !validateTribbles(&tribbles, numTargets)
		case 4: //client.PostTribble
			tribVal := userNum + tribIndex*numTargets
			msg := fmt.Sprintf("%d;%s", tribVal, *clientId)
			status, err := client.PostTribble(user, msg)
			if err != nil || status == tribrpc.NoSuchUser {
				failTest("error with PostTribble")
			}
			tribIndex++
		case 5: //client.GetTribblesBySubscription
			tribbles, status, err := client.GetTribblesBySubscription(user)
			if err != nil || status == tribrpc.NoSuchUser {
				failTest("error with GetTribblesBySubscription")
			}
			failed = !validateTribbles(&tribbles, numTargets)
		}
		if failed {
			failTest("tribbler output invalid")
		}
	}

	fmt.Println("PASS")
	os.Exit(7)
}

func failTest(msg string) {
	log.Fatalln("FAIL:", msg)
}

// Check if there are any duplicates in the returned subscriptions
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
