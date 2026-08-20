.PHONY: build install vet test lint fmt fix tidy all clean

ifeq ($(OS),Windows_NT)
    BIN_EXT := .exe
    RMDIR   := cmd.exe /C rmdir /S /Q
    DATE    := $(shell powershell -NoProfile -Command "Get-Date -Format 'yyyy-MM-ddTHH:mm:ssZ'")
    COMMIT  := $(shell git rev-parse --short HEAD 2>nul)
    VERSION := $(shell git describe --tags --always --dirty 2>nul)
else
    BIN_EXT :=
    RMDIR   := rm -rf
    DATE    := $(shell date -u +'%Y-%m-%dT%H:%M:%SZ' 2>/dev/null || echo "unknown")
    COMMIT  := $(shell git rev-parse --short HEAD 2>/dev/null || echo "none")
    VERSION := $(shell git describe --tags --always --dirty 2>/dev/null || echo "dev")
endif

ifeq ($(strip $(VERSION)),)
    VERSION := dev
endif
ifeq ($(strip $(COMMIT)),)
    COMMIT := none
endif
ifeq ($(strip $(DATE)),)
    DATE := unknown
endif

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
	go build -trimpath -ldflags="$(LDFLAGS)" -o bin/robo$(BIN_EXT) main.go

install:
	go install -trimpath -ldflags="$(LDFLAGS)" .

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
ifeq ($(OS),Windows_NT)
	-@if exist bin $(RMDIR) bin
else
	-@$(RMDIR) bin
endif
