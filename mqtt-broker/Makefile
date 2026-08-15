# Basic Makefile for Golang project
# Includes GRPC Gateway, Protocol Buffers
SERVICE		?= $(shell basename `go list`)
VERSION		?= $(shell git describe --tags --always --dirty --match=v* 2> /dev/null || cat $(PWD)/.version 2> /dev/null || echo v0)
PACKAGE		?= $(shell go list)
PACKAGES	?= $(shell go list ./...)
FILES		?= $(shell find . -type f -name '*.go' -not -path "./vendor/*")
BUILD_DIR	?= build

# Binaries
PROTOC		?= protoc

.PHONY: help clean fmt lint vet test test-core test-plugin test-broker test-cover build build-windows build-docker all

default: help

# show this help
help:
	@echo 'usage: make [target] ...'
	@echo ''
	@echo 'targets:'
	@egrep '^(.+)\:\ .*#\ (.+)' ${MAKEFILE_LIST} | sed 's/:.*#/#/' | column -t -c 2 -s '#'

# clean, format, build and unit test
all:
	make clean-all
	make gofmt
	make build
	make test

# build and install go application executable
install:
	go install -v ./...

# Print useful environment variables to stdout
env:
	echo $(CURDIR)
	echo $(SERVICE)
	echo $(PACKAGE)
	echo $(VERSION)

# go clean
clean:
	go clean

# remove all generated artifacts and clean all build artifacts
clean-all:
	go clean -i ./...
	rm -fr build

# fetch and install all required tools
tools:
	go get -u golang.org/x/tools/cmd/goimports
	go get -u github.com/golang/lint/golint
	go install github.com/golang/mock/mockgen@v1.6.0
	# Reqire when adding proto and grpc compilation
	# go get -u github.com/golang/protobuf/protoc-gen-go
	# go get -u github.com/mwitkow/go-proto-validators/protoc-gen-govalidators
	# go get -u github.com/grpc-ecosystem/grpc-gateway/protoc-gen-grpc-gateway
	# go get -u github.com/grpc-ecosystem/grpc-gateway/protoc-gen-swagger

# format the go source files
fmt:
	goimports -w $(FILES)

# run go lint on the source files
lint:
	golint $(PACKAGES)

 # run go vet on the source files
vet:
	go vet ./...

# generate godocs and start a local documentation webserver on port 8085
doc:
	godoc -http=:8085 -index

# update golang dependencies
update-dependencies:
	go mod tidy

# Generate protobuf, gRPC gateway, and Swagger artifacts.
# This is an optional maintainer workflow: committed generated files keep normal
# builds independent from protoc, while proto changes require the external tools.
generate-grpc: generate-proto

generate-proto:
	@command -v protoc >/dev/null 2>&1 || { echo 'external blocker: protoc is required for proto regeneration' >&2; exit 1; }
	@command -v protoc-gen-go >/dev/null 2>&1 || { echo 'external blocker: protoc-gen-go is required for proto regeneration' >&2; exit 1; }
	@command -v protoc-gen-go-grpc >/dev/null 2>&1 || { echo 'external blocker: protoc-gen-go-grpc is required for proto regeneration' >&2; exit 1; }
	@command -v protoc-gen-grpc-gateway >/dev/null 2>&1 || { echo 'external blocker: protoc-gen-grpc-gateway is required for proto regeneration' >&2; exit 1; }
	@command -v protoc-gen-swagger >/dev/null 2>&1 || { echo 'external blocker: protoc-gen-swagger is required for proto regeneration' >&2; exit 1; }
	@for script in plugin/admin/protos/proto_gen.sh plugin/auth/protos/proto_gen.sh plugin/federation/protos/proto_gen.sh; do \
		sh "$$script"; \
	done

# Compatibility aliases retained for existing maintainer workflows. Each alias
# regenerates the complete artifact set because the legacy scripts do not expose
# safe per-artifact phases.
compile-proto generate-grpcgw generate-swagger: generate-proto

# generate mock code
generate-mocks:
	@./mock_gen.sh

go-generate:
	go generate ./...

# Normal run/build/test targets consume committed generated artifacts and must
# not rewrite the checkout. Maintainers regenerate explicitly when sources change.
run:
	go run ./cmd/gmqttd start -c ./cmd/gmqttd/default_config.yml

build:
	go build -o $(BUILD_DIR)/gmqttd ./cmd/gmqttd

test:
	go test -v -race ./...

test-core:
	go test -v ./pkg/... ./config/... ./persistence/... ./server/...

test-plugin:
	go test -v ./plugin/aetherlink/... ./plugin/prometheus/...

test-broker:
	go test -v ./cmd/gmqttd/... ./cmd/gmqttd/command/...

build-windows:
	go build -o $(BUILD_DIR)/gmqttd.exe ./cmd/gmqttd

# run benchmark tests
test-bench:
	go test -bench ./...

# Generate test coverage
# Run test coverage and generate html report
test-cover:
	rm -fr coverage
	mkdir coverage
	go list -f '{{if gt (len .TestGoFiles) 0}}"go test -covermode count -coverprofile {{.Name}}.coverprofile -coverpkg ./... {{.ImportPath}}"{{end}}' ./... | xargs -I {} bash -c {}
	echo "mode: count" > coverage/cover.out
	grep -h -v "^mode:" *.coverprofile >> "coverage/cover.out"
	rm *.coverprofile
	go tool cover -html=coverage/cover.out -o=coverage/cover.html

test-all: test test-bench test-cover

# Build Golang application binary with settings to enable it to run in a Docker scratch container.
binary:
	CGO_ENABLED=0 GOOS=linux go build  -ldflags '-s' -o $(BUILD_DIR)/gmqttd ./cmd/gmqttd

build-docker:
	docker build -t gmqtt/gmqttd .
