// A simple program that the staff tests use to test your libstore
// implementation. You may use this program for your own testing purposes
// if you wish. DO NOT MODIFY!

package main

import (
	"flag"
	"fmt"
	"log"
	"net"
	"net/http"
	"net/rpc"
	"os"
	"strconv"
	"time"

	"github.com/cmu440/tribbler/libstore"
)

var (
	forceLease    = flag.Bool("fl", false, "Create libstore in 'Always' mode (default is 'Normal')")
	serverAddress = flag.String("host", "localhost", "master storage server host (our tests will always use localhost)")
	port          = flag.Int("port", 9009, "master storage server port number")
	numTimes      = flag.Int("n", 1, "number of times to execute the command")
	handleLeases  = flag.Bool("l", false, "run persistently, requesting leases, and reporting lease revocation requests")
)

func init() {
	log.SetFlags(log.Lshortfile | log.Lmicroseconds)
	flag.Usage = func() {
		fmt.Fprintln(os.Stderr, "The lrunner program is a testing tool that that creates and runs an instance")
		fmt.Fprintln(os.Stderr, "of your Libstore. You may use it to test the correctness of your storage server.\n")
		fmt.Fprintln(os.Stderr, "Usage:")
		flag.PrintDefaults()
		fmt.Fprintln(os.Stderr)
		fmt.Fprintln(os.Stderr, "Possible commands:")
		fmt.Fprintln(os.Stderr, "  Put:            p  key value")
		fmt.Fprintln(os.Stderr, "  Get:            g  key")
		fmt.Fprintln(os.Stderr, "  GetList:        lg key")
		fmt.Fprintln(os.Stderr, "  AddToList:      la key value")
		fmt.Fprintln(os.Stderr, "  RemoveFromList: lr key value")
	}
}

type cmdInfo struct {
	cmdline string
	nargs   int
}

var cmdList = map[string]int{
	"p":  2,
	"g":  1,
	"la": 2,
	"lr": 2,
	"lg": 1,
}

func main() {
	flag.Parse()
	if flag.NArg() < 2 {
		flag.Usage()
		os.Exit(1)
	}

	cmdmap := make(map[string]cmdInfo)
	for k, v := range cmdList {
		cmdmap[k] = cmdInfo{cmdline: k, nargs: v}
	}

	cmd := flag.Arg(0)
	ci, found := cmdmap[cmd]
	if !found {
		flag.Usage()
		os.Exit(1)
	}
	if flag.NArg() < (ci.nargs + 1) {
		flag.Usage()
		os.Exit(1)
	}

	var leaseCallbackAddr string
	if *handleLeases {
		// Setup an HTTP handler to receive remote lease revocation requests.
		// The student's libstore implementation is resonsible for calling
		// rpc.RegisterName("LeaseCallbacks", librpc.Wrap(libstore)) to finish
		// the setup.
		l, err := net.Listen("tcp", ":0")
		if err != nil {
			log.Fatalln("Failed to listen:", err)
		}
		_, listenPort, _ := net.SplitHostPort(l.Addr().String())
		leaseCallbackAddr = net.JoinHostPort("localhost", listenPort)
		rpc.HandleHTTP()
		go http.Serve(l, nil)
	}

	var leaseMode libstore.LeaseMode
	if *handleLeases && *forceLease {
		leaseMode = libstore.Always
	} else if leaseCallbackAddr == "" {
		leaseMode = libstore.Never
	} else {
		leaseMode = libstore.Normal
	}

	masterHostPort := net.JoinHostPort(*serverAddress, strconv.Itoa(*port))
	ls, err := libstore.NewLibstore(masterHostPort, leaseCallbackAddr, leaseMode)
	if err != nil {
		log.Fatalln("Failed to create libstore:", err)
	}

	for i := 0; i < *numTimes; i++ {
		switch cmd {
		case "g":
			val, err := ls.Get(flag.Arg(1))
			if err != nil {
				fmt.Println("ERROR:", err)
			} else {
				fmt.Println(val)
			}
		case "lg":
			val, err := ls.GetList(flag.Arg(1))
			if err != nil {
				fmt.Println("ERROR:", err)
			} else {
				for _, i := range val {
					fmt.Println(i)
				}
			}
		case "p", "la", "lr":
			var err error
			switch cmd {
			case "p":
				err = ls.Put(flag.Arg(1), flag.Arg(2))
			case "la":
				err = ls.AppendToList(flag.Arg(1), flag.Arg(2))
			case "lr":
				err = ls.RemoveFromList(flag.Arg(1), flag.Arg(2))
			}
			if err == nil {
				fmt.Println("OK")
			} else {
				fmt.Println("ERROR:", err)
			}
		}
	}

	if *handleLeases {
		fmt.Println("Waiting 20 seconds for lease callbacks...")
		time.Sleep(20 * time.Second)
	}
}
