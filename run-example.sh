#!/bin/sh -e
go install github.com/YuriyNasretdinov/hotreload/cmd/soft

plugpath="hotreload/cmd/example/plug/plug.so"
soft=$(go env GOPATH)/bin/soft

$soft sh -c 'set -e -x;\
    go build -buildmode=plugin -o '$plugpath' github.com/YuriyNasretdinov/hotreload/cmd/example/plug;\
    go run cmd/example/main.go -plug='$plugpath';\
    go test ./cmd/example/example_test.go'
