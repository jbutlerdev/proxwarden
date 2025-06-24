# ProxWarden Makefile

# Build variables
BINARY_NAME=proxwarden
BINARY_PATH=./$(BINARY_NAME)
VERSION?=dev
GIT_COMMIT?=$(shell git rev-parse --short HEAD 2>/dev/null || echo "unknown")
BUILD_DATE?=$(shell date -u +"%Y-%m-%dT%H:%M:%SZ")
LDFLAGS=-ldflags "-X github.com/jbutlerdev/proxwarden/cmd/proxwarden.Version=$(VERSION) -X github.com/jbutlerdev/proxwarden/cmd/proxwarden.GitCommit=$(GIT_COMMIT) -X github.com/jbutlerdev/proxwarden/cmd/proxwarden.BuildDate=$(BUILD_DATE)"

# Go parameters
GOCMD=go
GOBUILD=$(GOCMD) build
GOCLEAN=$(GOCMD) clean
GOTEST=$(GOCMD) test
GOGET=$(GOCMD) get
GOMOD=$(GOCMD) mod
GOFMT=gofmt
GOLINT=golangci-lint

# Directories
BUILD_DIR=build
DIST_DIR=dist

.PHONY: all build clean test test-coverage test-race deps fmt lint vet check install uninstall help

## all: Default target - builds the binary
all: build

## build: Build the binary
build:
	@echo "Building $(BINARY_NAME)..."
	$(GOBUILD) $(LDFLAGS) -o $(BINARY_PATH) .

## build-linux: Build for Linux
build-linux:
	@echo "Building $(BINARY_NAME) for Linux..."
	GOOS=linux GOARCH=amd64 $(GOBUILD) $(LDFLAGS) -o $(BUILD_DIR)/$(BINARY_NAME)-linux-amd64 .

## build-all: Build for multiple platforms
build-all: clean
	@echo "Building for multiple platforms..."
	@mkdir -p $(BUILD_DIR)
	GOOS=linux GOARCH=amd64 $(GOBUILD) $(LDFLAGS) -o $(BUILD_DIR)/$(BINARY_NAME)-linux-amd64 .
	GOOS=linux GOARCH=arm64 $(GOBUILD) $(LDFLAGS) -o $(BUILD_DIR)/$(BINARY_NAME)-linux-arm64 .
	GOOS=darwin GOARCH=amd64 $(GOBUILD) $(LDFLAGS) -o $(BUILD_DIR)/$(BINARY_NAME)-darwin-amd64 .
	GOOS=darwin GOARCH=arm64 $(GOBUILD) $(LDFLAGS) -o $(BUILD_DIR)/$(BINARY_NAME)-darwin-arm64 .

## clean: Clean build artifacts
clean:
	@echo "Cleaning..."
	$(GOCLEAN)
	rm -f $(BINARY_PATH)
	rm -rf $(BUILD_DIR)
	rm -rf $(DIST_DIR)

## test: Run all tests
test:
	@echo "Running tests..."
	$(GOTEST) -v ./...

## test-coverage: Run tests with coverage
test-coverage:
	@echo "Running tests with coverage..."
	$(GOTEST) -race -coverprofile=coverage.out -covermode=atomic ./...
	$(GOCMD) tool cover -html=coverage.out -o coverage.html
	@echo "Coverage report generated: coverage.html"

## test-race: Run tests with race detection
test-race:
	@echo "Running tests with race detection..."
	$(GOTEST) -race ./...

## deps: Download dependencies
deps:
	@echo "Downloading dependencies..."
	$(GOGET) github.com/luthermonson/go-proxmox@latest
	$(GOGET) github.com/spf13/cobra@latest
	$(GOGET) github.com/spf13/viper@latest
	$(GOGET) github.com/sirupsen/logrus@latest
	$(GOGET) go.uber.org/zap@latest
	$(GOGET) gopkg.in/yaml.v3@latest
	$(GOMOD) tidy

## fmt: Format code
fmt:
	@echo "Formatting code..."
	$(GOFMT) -s -w .

## lint: Run linter
lint:
	@echo "Running linter..."
	@which $(GOLINT) > /dev/null || (echo "Installing golangci-lint..." && curl -sSfL https://raw.githubusercontent.com/golangci/golangci-lint/master/install.sh | sh -s -- -b $(shell go env GOPATH)/bin v1.54.2)
	$(GOLINT) run

## vet: Run go vet
vet:
	@echo "Running go vet..."
	$(GOCMD) vet ./...

## check: Run all checks (fmt, vet, lint, test)
check: fmt vet lint test

## install: Install the binary and systemd service
install: build
	@echo "Installing ProxWarden..."
	sudo ./scripts/install.sh

## uninstall: Uninstall ProxWarden
uninstall:
	@echo "Uninstalling ProxWarden..."
	sudo systemctl stop proxwarden || true
	sudo systemctl disable proxwarden || true
	sudo rm -f /etc/systemd/system/proxwarden.service
	sudo rm -f /usr/local/bin/proxwarden
	sudo rm -rf /etc/proxwarden
	sudo userdel proxwarden || true
	sudo groupdel proxwarden || true
	sudo systemctl daemon-reload

## release: Create a release build
release: VERSION ?= $(shell git describe --tags --abbrev=0 2>/dev/null || echo "v0.1.0")
release: clean build-all
	@echo "Creating release $(VERSION)..."
	@mkdir -p $(DIST_DIR)
	@for binary in $(BUILD_DIR)/*; do \
		if [ -f "$$binary" ]; then \
			filename=$$(basename $$binary); \
			tar -czf $(DIST_DIR)/$$filename.tar.gz -C $(BUILD_DIR) $$filename; \
		fi; \
	done
	@echo "Release archives created in $(DIST_DIR)/"

## run: Build and run the daemon
run: build
	@echo "Starting ProxWarden daemon..."
	./$(BINARY_PATH) daemon

## dev: Run in development mode with debug logging
dev: build
	@echo "Starting ProxWarden in development mode..."
	./$(BINARY_PATH) daemon --debug --log-level debug

## help: Show this help message
help:
	@echo "ProxWarden Makefile"
	@echo ""
	@echo "Usage: make [target]"
	@echo ""
	@echo "Targets:"
	@awk '/^##/{c=substr($$0,3);next}c&&/^[[:alpha:]][[:alnum:]_-]+:/{print substr($$1,1,index($$1,":")),c}1{c=""}' $(MAKEFILE_LIST) | column -t -s: | sort

# Default target
.DEFAULT_GOAL := help