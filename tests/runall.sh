#!/bin/bash

if [ -z $GOPATH ]; then
    echo "WARNING! GOPATH environment variable is not set!"
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

$GOPATH/tests/tribtest.sh
$GOPATH/tests/libtest.sh
$GOPATH/tests/libtest2.sh
$GOPATH/tests/storagetest.sh
$GOPATH/tests/storagetest2.sh
$GOPATH/tests/stresstest.sh