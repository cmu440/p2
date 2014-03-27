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


# Build student binaries. Exit immediately if there was a compile-time error.
go install github.com/cmu440/tribbler/runners/trunner
if [ $? -ne 0 ]; then
   echo "FAIL: code does not compile"
   exit $?
fi
go install github.com/cmu440/tribbler/runners/srunner
if [ $? -ne 0 ]; then
   echo "FAIL: code does not compile"
   exit $?
fi
go install github.com/cmu440/tribbler/tests/stresstest
if [ $? -ne 0 ]; then
   echo "FAIL: code does not compile"
   exit $?
fi

# Pick random port between [10000, 20000).
STORAGE_PORT=$(((RANDOM % 10000) + 10000))
STORAGE_SERVER=$GOPATH/bin/srunner
STRESS_CLIENT=$GOPATH/bin/stresstest
TRIB_SERVER=$GOPATH/bin/trunner

function startStorageServers {
    N=${#STORAGE_ID[@]}
    # Start master storage server.
    ${STORAGE_SERVER} -N=${N} -id=${STORAGE_ID[0]} -port=${STORAGE_PORT} &> /dev/null &
    STORAGE_SERVER_PID[0]=$!
    # Start slave storage servers.
    if [ "$N" -gt 1 ]
    then
        for i in `seq 1 $((N - 1))`
        do
	    STORAGE_SLAVE_PORT=$(((RANDOM % 10000) + 10000))
            ${STORAGE_SERVER} -port=${STORAGE_SLAVE_PORT} -id=${STORAGE_ID[$i]} -master="localhost:${STORAGE_PORT}" &> /dev/null &
            STORAGE_SERVER_PID[$i]=$!
        done
    fi
    sleep 5
}

function stopStorageServers {
    N=${#STORAGE_ID[@]}
    for i in `seq 0 $((N - 1))`
    do
        kill -9 ${STORAGE_SERVER_PID[$i]}
        wait ${STORAGE_SERVER_PID[$i]} 2> /dev/null
    done
}

function startTribServers {
    for i in `seq 0 $((M - 1))`
    do
        # Pick random port between [10000, 20000).
        TRIB_PORT[$i]=$(((RANDOM % 10000) + 10000))
        ${TRIB_SERVER} -port=${TRIB_PORT[$i]} "localhost:${STORAGE_PORT}" &> /dev/null &
        TRIB_SERVER_PID[$i]=$!
    done
    sleep 5
}

function stopTribServers {
    for i in `seq 0 $((M - 1))`
    do
        kill -9 ${TRIB_SERVER_PID[$i]}
        wait ${TRIB_SERVER_PID[$i]} 2> /dev/null
    done
}

function testStress {
    echo "Starting ${#STORAGE_ID[@]} storage server(s)..."
    startStorageServers
    echo "Starting ${M} Tribble server(s)..."
    startTribServers
    # Start stress clients
    C=0
    K=${#CLIENT_COUNT[@]}
    for USER in `seq 0 $((K - 1))`
    do
        for CLIENT in `seq 0 $((CLIENT_COUNT[$USER] - 1))`
        do
            ${STRESS_CLIENT} -port=${TRIB_PORT[$((C % M))]} -clientId=${CLIENT} ${USER} ${K} & 
            STRESS_CLIENT_PID[$C]=$!
            # Setup background thread to kill client upon timeout.
            sleep ${TIMEOUT} && kill -9 ${STRESS_CLIENT_PID[$C]} &> /dev/null &
            C=$((C + 1))
        done
    done
    echo "Running ${C} client(s)..."

    # Check exit status.
    FAIL=0
    for i in `seq 0 $((C - 1))`
    do
        wait ${STRESS_CLIENT_PID[$i]} 2> /dev/null
        if [ "$?" -ne 7 ]
        then
            FAIL=$((FAIL + 1))
        fi
    done
    if [ "$FAIL" -eq 0 ]
    then
        echo "PASS"
        PASS_COUNT=$((PASS_COUNT + 1))
    else
        echo "FAIL: ${FAIL} clients failed"
        FAIL_COUNT=$((FAIL_COUNT + 1))
    fi
    stopTribServers
    stopStorageServers
    sleep 1
}

# Testing single client, single tribserver, single storageserver.
function testStressSingleClientSingleTribSingleStorage {
    echo "Running testStressSingleClientSingleTribSingleStorage:"
    STORAGE_ID=('0')
    M=1
    CLIENT_COUNT=('1')
    TIMEOUT=15
    testStress
}

# Testing single client, single tribserver, multiple storageserver.
function testStressSingleClientSingleTribMultipleStorage {
    echo "Running testStressSingleClientSingleTribMultipleStorage:"
    STORAGE_ID=('0' '0' '0')
    M=1
    CLIENT_COUNT=('1')
    TIMEOUT=15
    testStress
}

# Testing multiple client, single tribserver, single storageserver.
function testStressMultipleClientSingleTribSingleStorage {
    echo "Running testStressMultipleClientSingleTribSingleStorage:"
    STORAGE_ID=('0')
    M=1
    CLIENT_COUNT=('1' '1' '1')
    TIMEOUT=15
    testStress
}

# Testing multiple client, single tribserver, multiple storageserver.
function testStressMultipleClientSingleTribMultipleStorage {
    echo "Running testStressMultipleClientSingleTribMultipleStorage:"
    STORAGE_ID=('0' '0' '0' '0' '0' '0')
    M=1
    CLIENT_COUNT=('1' '1' '1')
    TIMEOUT=15
    testStress
}

# Testing multiple client, multiple tribserver, single storageserver.
function testStressMultipleClientMultipleTribSingleStorage {
    echo "Running testStressMultipleClientMultipleTribSingleStorage:"
    STORAGE_ID=('0')
    M=2
    CLIENT_COUNT=('1' '1')
    TIMEOUT=30
    testStress
}

# Testing multiple client, multiple tribserver, multiple storageserver.
function testStressMultipleClientMultipleTribMultipleStorage {
    echo "Running testStressMultipleClientMultipleTribMultipleStorage:"
    STORAGE_ID=('0' '0' '0' '0' '0' '0' '0')
    M=3
    CLIENT_COUNT=('1' '1' '1')
    TIMEOUT=30
    testStress
}

# Testing 2x more clients than tribservers, multiple tribserver, multiple storageserver.
function testStressDoubleClientMultipleTribMultipleStorage {
    echo "Running testStressDoubleClientMultipleTribMultipleStorage:"
    STORAGE_ID=('0' '0' '0' '0' '0' '0')
    M=2
    CLIENT_COUNT=('1' '1' '1' '1')
    TIMEOUT=30
    testStress
}


# Testing duplicate users, multiple tribserver, single storageserver.
function testStressDupUserMultipleTribSingleStorage {
    echo "Running testStressDupUserMultipleTribSingleStorage:"
    STORAGE_ID=('0')
    M=2
    CLIENT_COUNT=('2')
    TIMEOUT=30
    testStress
}

# Testing duplicate users, multiple tribserver, multiple storageserver.
function testStressDupUserMultipleTribMultipleStorage {
    echo "Running testStressDupUserMultipleTribMultipleStorage:"
    STORAGE_ID=('0' '0' '0')
    M=2
    CLIENT_COUNT=('2')
    TIMEOUT=30
    testStress
}

# Run tests.
PASS_COUNT=0
FAIL_COUNT=0
testStressSingleClientSingleTribSingleStorage
testStressSingleClientSingleTribMultipleStorage
testStressMultipleClientSingleTribSingleStorage
testStressMultipleClientSingleTribMultipleStorage
testStressMultipleClientMultipleTribSingleStorage
testStressMultipleClientMultipleTribMultipleStorage
testStressDoubleClientMultipleTribMultipleStorage
testStressDupUserMultipleTribSingleStorage
testStressDupUserMultipleTribMultipleStorage

echo "Passed (${PASS_COUNT}/$((PASS_COUNT + FAIL_COUNT))) tests"
