.PHONY: build install vet test lint fmt fix tidy all clean

VERSION ?= $(shell git describe --tags --always --dirty 2>/dev/null || echo "dev")
COMMIT  ?= $(shell git rev-parse --short HEAD 2>/dev/null || echo "none")
DATE    ?= $(shell date -u +'%Y-%m-%dT%H:%M:%SZ' 2>/dev/null || echo "unknown")

LDFLAGS := -s -w \
           -X github.com/vladimirvivien/robo/cmd.Version=$(VERSION) \
           -X github.com/vladimirvivien/robo/cmd.Commit=$(COMMIT) \
           -X github.com/vladimirvivien/robo/cmd.BuildDate=$(DATE)

# Default target: everything CI runs, locally.
all: fix fmt vet test lint build

fix:
	go fix ./...

fmt:
	go fmt ./...

build:
	CGO_ENABLED=0 go build -trimpath -ldflags="$(LDFLAGS)" -o bin/robo main.go

install:
	CGO_ENABLED=0 go install -trimpath -ldflags="$(LDFLAGS)" .

vet:
	go vet ./...

test:
	go test -race ./...

lint:
	golangci-lint run ./...

tidy:
	go mod tidy

clean:
	go clean ./...
	rm -rf bin/
