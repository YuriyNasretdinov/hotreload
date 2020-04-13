#!/bin/sh
set -e

export GOPATH=$(go env GOPATH)

go install github.com/YuriyNasretdinov/hotreload/cmd/hot

$GOPATH/bin/hot \
    -watch=$GOPATH/src/github.com/YuriyNasretdinov/hotreload/cmd/live \
    sh -c 'set -e -x;\
    go install -v github.com/YuriyNasretdinov/hotreload/cmd/live;\
    $GOPATH/bin/live'
