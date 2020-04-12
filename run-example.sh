#!/bin/sh -e
go install github.com/YuriyNasretdinov/hotreload/cmd/hot

plugpath="plug.so"

$(go env GOPATH)/bin/hot sh -c 'set -e -x;\
    go build -buildmode=plugin -o '$plugpath' github.com/YuriyNasretdinov/hotreload/cmd/example/plug;\
    go run cmd/example/main.go -plug='$plugpath';\
    go test ./cmd/example/example_test.go'
