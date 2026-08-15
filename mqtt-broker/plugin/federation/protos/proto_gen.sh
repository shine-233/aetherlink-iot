#!/usr/bin/env sh
set -eu

SCRIPT_DIR=$(CDPATH= cd -- "$(dirname -- "$0")" && pwd)
cd "$SCRIPT_DIR"

protoc -I. \
-I$GOPATH/src/github.com/grpc-ecosystem/grpc-gateway \
-I$GOPATH/src/github.com/grpc-ecosystem/grpc-gateway/third_party/googleapis \
--go-grpc_out=../ \
--go_out=../ \
--grpc-gateway_out=../ \
--swagger_out=../swagger \
*.proto
