# Makefile — song-reviewer
#
# Available targets:
#   build    Compile the binary to ./song-reviewer
#   test     Run the full test suite
#   install  Install the binary to $GOPATH/bin
#   lint     Run go vet (and staticcheck if available)
#
# VERSION defaults to "dev"; override with: make build VERSION=1.2.3

VERSION ?= dev

.PHONY: build test install lint

build:
	go build -ldflags "-X main.version=$(VERSION)" -o song-reviewer ./cmd/reviewer

test:
	go test ./...

install:
	go install -ldflags "-X main.version=$(VERSION)" ./cmd/reviewer

lint:
	go vet ./...
	@if command -v staticcheck > /dev/null 2>&1; then staticcheck ./...; fi
