#!/bin/bash

if [ -z $GOPATH ]; then
    echo "FAIL: GOPATH environment variable is not set"
    exit 1
fi

if [ -n "$(go version | grep 'darwin/amd64')" ]; then    
    GOOS="darwin_amd64"
elif [ -n "$(go version | grep 'linux/amd64')" ]; then
    GOOS="linux_amd64"
else
    echo "FAIL: only 64-bit Mac OS X and Linux operating systems are supported"
    exit 1
fi

# Build the test binary to use to test the student's tribble server implementation.
# Exit immediately if there was a compile-time error.
go install github.com/cmu440/tribbler/tests/tribtest
if [ $? -ne 0 ]; then
   echo "FAIL: code does not compile"
   exit $?
fi

# Pick random ports between [10000, 20000).
STORAGE_PORT=$(((RANDOM % 10000) + 10000))
TRIB_PORT=$(((RANDOM % 10000) + 10000))
STORAGE_SERVER=$GOPATH/sols/$GOOS/srunner
TRIBTEST=$GOPATH/bin/tribtest

# Start an instance of the staff's official storage server implementation.
${STORAGE_SERVER} -port=${STORAGE_PORT} 2> /dev/null &
STORAGE_SERVER_PID=$!
sleep 5

# Start the test.
${TRIBTEST} -port=${TRIB_PORT} "localhost:${STORAGE_PORT}"

# Kill the storage server.
kill -9 ${STORAGE_SERVER_PID}
wait ${STORAGE_SERVER_PID} 2> /dev/null
