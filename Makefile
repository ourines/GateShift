BINARY_NAME=proxy
VERSION=$(shell git describe --tags --always --dirty 2>/dev/null || echo "dev")
BUILD_TIME=$(shell date -u +"%Y-%m-%dT%H:%M:%SZ")
LDFLAGS=-ldflags "-X main.Version=$(VERSION) -X main.BuildTime=$(BUILD_TIME)"

.PHONY: all build clean test install uninstall

# Default target
all: build

# Build binary
build:
	go build $(LDFLAGS) -o bin/$(BINARY_NAME) ./cmd/proxy

# Cross compile for all platforms
build-all: build-linux build-darwin build-windows

# Build for Linux
build-linux:
	GOOS=linux GOARCH=amd64 go build $(LDFLAGS) -o bin/$(BINARY_NAME)_linux_amd64 ./cmd/proxy
	GOOS=linux GOARCH=arm64 go build $(LDFLAGS) -o bin/$(BINARY_NAME)_linux_arm64 ./cmd/proxy

# Build for macOS
build-darwin:
	GOOS=darwin GOARCH=amd64 go build $(LDFLAGS) -o bin/$(BINARY_NAME)_darwin_amd64 ./cmd/proxy
	GOOS=darwin GOARCH=arm64 go build $(LDFLAGS) -o bin/$(BINARY_NAME)_darwin_arm64 ./cmd/proxy

# Build for Windows
build-windows:
	GOOS=windows GOARCH=amd64 go build $(LDFLAGS) -o bin/$(BINARY_NAME)_windows_amd64.exe ./cmd/proxy

# Run tests
test:
	go test -v ./...

# Clean build files
clean:
	rm -rf bin/

# Install the application locally
install: build
	mkdir -p $(HOME)/bin
	cp bin/$(BINARY_NAME) $(HOME)/bin/

# Uninstall the application
uninstall:
	rm -f $(HOME)/bin/$(BINARY_NAME) 