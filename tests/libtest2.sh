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

# Build the lrunner binary to use to test the student's libstore implementation.
# Exit immediately if there was a compile-time error.
go install github.com/cmu440/tribbler/runners/lrunner
if [ $? -ne 0 ]; then
   echo "FAIL: code does not compile"
   exit $?
fi

# Pick random port between [10000, 20000).
STORAGE_PORT=$(((RANDOM % 10000) + 10000))
STORAGE_SERVER=$GOPATH/sols/$GOOS/srunner
LRUNNER=$GOPATH/bin/lrunner

function startStorageServers {
    N=${#STORAGE_ID[@]}
    # Start master storage server.
    ${STORAGE_SERVER} -N=${N} -id=${STORAGE_ID[0]} -port=${STORAGE_PORT} 2> /dev/null &
    STORAGE_SERVER_PID[0]=$!
    # Start slave storage servers.
    if [ "$N" -gt 1 ]
    then
        for i in `seq 1 $((N-1))`
        do
	    STORAGE_SLAVE_PORT=$(((RANDOM % 10000) + 10000))
            ${STORAGE_SERVER} -id=${STORAGE_ID[$i]} -port=${STORAGE_SLAVE_PORT} -master="localhost:${STORAGE_PORT}" 2> /dev/null &
            STORAGE_SERVER_PID[$i]=$!
        done
    fi
    sleep 5
}

function stopStorageServers {
    N=${#STORAGE_ID[@]}
    for i in `seq 0 $((N-1))`
    do
        kill -9 ${STORAGE_SERVER_PID[$i]}
        wait ${STORAGE_SERVER_PID[$i]} 2> /dev/null
    done
}

# Testing delayed start.
function testDelayedStart {
    echo "Running testDelayedStart:"

    # Start master storage server.
    ${STORAGE_SERVER} -N=2 -port=${STORAGE_PORT} 2> /dev/null &
    STORAGE_SERVER_PID1=$!
    sleep 5

    # Run lrunner.
    ${LRUNNER} -port=${STORAGE_PORT} p "key:" value &> /dev/null &
    sleep 3

    # Start second storage server.
    STORAGE_SLAVE_PORT=$(((RANDOM % 10000) + 10000))
    ${STORAGE_SERVER} -master="localhost:${STORAGE_PORT}" -port=${STORAGE_SLAVE_PORT} 2> /dev/null &
    STORAGE_SERVER_PID2=$!
    sleep 5

    # Run lrunner.
    PASS=`${LRUNNER} -port=${STORAGE_PORT} g "key:" | grep value | wc -l`
    if [ "$PASS" -eq 1 ]
    then
        echo "PASS"
        PASS_COUNT=$((PASS_COUNT + 1))
    else
        echo "FAIL"
        FAIL_COUNT=$((FAIL_COUNT + 1))
    fi

    # Kill storage servers.
    kill -9 ${STORAGE_SERVER_PID1}
    kill -9 ${STORAGE_SERVER_PID2}
    wait ${STORAGE_SERVER_PID1} 2> /dev/null
    wait ${STORAGE_SERVER_PID2} 2> /dev/null
}

function testRouting {
    startStorageServers
    for KEY in "${KEYS[@]}"
    do
        ${LRUNNER} -port=${STORAGE_PORT} p ${KEY} value > /dev/null
        PASS=`${LRUNNER} -port=${STORAGE_PORT} g ${KEY} | grep value | wc -l`
        if [ "$PASS" -ne 1 ]
        then
            break
        fi
    done
    if [ "$PASS" -eq 1 ]
    then
        echo "PASS"
        PASS_COUNT=$((PASS_COUNT + 1))
    else
        echo "FAIL"
        FAIL_COUNT=$((FAIL_COUNT + 1))
    fi
    stopStorageServers
}

# Testing routing general.
function testRoutingGeneral {
    echo "Running testRoutingGeneral:"
    STORAGE_ID=('3000000000' '4000000000' '2000000000')
    KEYS=('bubble:' 'insertion:' 'merge:' 'heap:' 'quick:' 'radix:')
    testRouting
}

# Testing routing wraparound.
function testRoutingWraparound {
    echo "Running testRoutingWraparound:"
    STORAGE_ID=('2000000000' '2500000000' '3000000000')
    KEYS=('bubble:' 'insertion:' 'merge:' 'heap:' 'quick:' 'radix:')
    testRouting
}

# Testing routing equal.
function testRoutingEqual {
    echo "Running testRoutingEqual:"
    STORAGE_ID=('3835649095' '1581790440' '2373009399' '3448274451' '1666346102' '2548238361')
    KEYS=('bubble:' 'insertion:' 'merge:' 'heap:' 'quick:' 'radix:')
    testRouting
}

# Run tests
PASS_COUNT=0
FAIL_COUNT=0
testDelayedStart
testRoutingGeneral
testRoutingWraparound
testRoutingEqual

echo "Passed (${PASS_COUNT}/$((PASS_COUNT + FAIL_COUNT))) tests"
