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

# Build the student's storage server implementation.
# Exit immediately if there was a compile-time error.
go install github.com/cmu440/tribbler/runners/srunner
if [ $? -ne 0 ]; then
   echo "FAIL: code does not compile"
   exit $?
fi

# Build the test binary to use to test the student's storage server implementation.
# Exit immediately if there was a compile-time error.
go install github.com/cmu440/tribbler/tests/storagetest
if [ $? -ne 0 ]; then
   echo "FAIL: code does not compile"
   exit $?
fi

# Pick random ports between [10000, 20000).
STORAGE_PORT=$(((RANDOM % 10000) + 10000))
TESTER_PORT=$(((RANDOM % 10000) + 10000))
STORAGE_TEST=$GOPATH/bin/storagetest
STORAGE_SERVER=$GOPATH/bin/srunner

##################################################

# Start storage server.
${STORAGE_SERVER} -port=${STORAGE_PORT} 2> /dev/null &
STORAGE_SERVER_PID=$!
sleep 5

# Start storagetest.
${STORAGE_TEST} -port=${TESTER_PORT} -type=2 "localhost:${STORAGE_PORT}"

# Kill storage server.
kill -9 ${STORAGE_SERVER_PID}
wait ${STORAGE_SERVER_PID} 2> /dev/null

##################################################

# Start storage server.
${STORAGE_SERVER} -port=${STORAGE_PORT} -N=2 -id=900 2> /dev/null &
STORAGE_SERVER_PID=$!
sleep 5

# Start storagetest.
${STORAGE_TEST} -port=${TESTER_PORT} -type=1 -N=2 -id=800 "localhost:${STORAGE_PORT}"

# Kill storage server.
kill -9 ${STORAGE_SERVER_PID}
wait ${STORAGE_SERVER_PID} 2> /dev/null
