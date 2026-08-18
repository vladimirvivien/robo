.PHONY: build vet test lint fmt fix tidy all clean

# Default target: everything CI runs, locally.
all: fix fmt build vet test lint

fix:
	go fix ./...

fmt:
	go fmt ./...

build:
	go build ./...

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
