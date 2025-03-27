# Project Name
APP_NAME = dbcp-agent

# Build output
BUILD_DIR = bin

# Default config
CONFIG = configs/agent-config.yaml

# Default Go version compatibility
GO_VERSION = 1.22

# Paths
SRC = ./cmd/$(APP_NAME)

# Default target
.PHONY: all
all: build

# Build the binary
.PHONY: build
build:
	go build -o $(BUILD_DIR)/$(APP_NAME) $(SRC)

# Run the agent
.PHONY: run
run: build
	./$(BUILD_DIR)/$(APP_NAME) $(CONFIG)

# Run tests
.PHONY: test
test:
	go test ./...

# Run tests with verbose output
.PHONY: test-verbose
test-verbose:
	go test -v ./...

# Clean build output
.PHONY: clean
clean:
	rm -rf $(BUILD_DIR)

# Generate a Linux release binary
.PHONY: release
release:
	GOOS=linux GOARCH=amd64 go build -ldflags="-s -w" -o $(BUILD_DIR)/$(APP_NAME) $(SRC)

# Lint (optional: if you use golangci-lint)
.PHONY: lint
lint:
	golangci-lint run ./...

# Format code (optional)
.PHONY: fmt
fmt:
	go fmt ./...
