# Makefile for grove-docgen (docgen)

BINARY_NAME=docgen
BIN_DIR=bin
VERSION_PKG=github.com/grovetools/core/version

# --- Versioning ---
GIT_COMMIT ?= $(shell git rev-parse --short HEAD)
GIT_BRANCH ?= $(shell git rev-parse --abbrev-ref HEAD)
GIT_DIRTY  ?= $(shell test -n "`git status --porcelain`" && echo "-dirty")
BUILD_DATE ?= $(shell date -u +%Y-%m-%dT%H:%M:%SZ)
VERSION ?= $(GIT_BRANCH)-$(GIT_COMMIT)$(GIT_DIRTY)

# Go LDFLAGS to inject version info at compile time
LDFLAGS = -ldflags="\
-X '$(VERSION_PKG).Version=$(VERSION)' \
-X '$(VERSION_PKG).Commit=$(GIT_COMMIT)' \
-X '$(VERSION_PKG).Branch=$(GIT_BRANCH)' \
-X '$(VERSION_PKG).BuildDate=$(BUILD_DATE)'"

.PHONY: all build test clean fmt vet lint run check dev build-all generate-docs schema help

all: build

schema:
	@echo "Generating JSON schema..."
	@go generate ./pkg/config

build: schema
	@mkdir -p $(BIN_DIR)
	@echo "Building $(BINARY_NAME) version $(VERSION)..."
	@go build $(LDFLAGS) -o $(BIN_DIR)/$(BINARY_NAME) .

test:
	@echo "Running tests..."
	@go test -v ./...

clean:
	@echo "Cleaning..."
	@go clean
	@rm -rf $(BIN_DIR)
	@rm -f $(BINARY_NAME)
	@rm -f coverage.out

fmt:
	@echo "Formatting code..."
	@go fmt ./...

vet:
	@echo "Running go vet..."
	@go vet ./...

lint:
	@echo "Running linter..."
	@if command -v golangci-lint > /dev/null; then \
		golangci-lint run; \
	else \
		echo "golangci-lint not installed. Install with: go install github.com/golangci/golangci-lint/cmd/golangci-lint@latest"; \
	fi

run: build
	@$(BIN_DIR)/$(BINARY_NAME) $(ARGS)

check: fmt vet lint test

dev:
	@mkdir -p $(BIN_DIR)
	@echo "Building $(BINARY_NAME) version $(VERSION) with race detector..."
	@go build -race $(LDFLAGS) -o $(BIN_DIR)/$(BINARY_NAME) .

PLATFORMS ?= darwin/amd64 darwin/arm64 linux/amd64 linux/arm64
DIST_DIR ?= dist

build-all:
	@echo "Building for multiple platforms into $(DIST_DIR)..."
	@mkdir -p $(DIST_DIR)
	@for platform in $(PLATFORMS); do \
		os=$$(echo $$platform | cut -d'/' -f1); \
		arch=$$(echo $$platform | cut -d'/' -f2); \
		output_name="$(BINARY_NAME)-$${os}-$${arch}"; \
		echo "  -> Building $${output_name} version $(VERSION)"; \
		GOOS=$$os GOARCH=$$arch go build $(LDFLAGS) -o $(DIST_DIR)/$${output_name} .; \
	done

# Generate documentation for grove-docgen itself
generate-docs: build
	@echo "Generating docgen documentation using docgen..."
	@$(BIN_DIR)/$(BINARY_NAME) generate
	@echo "Synchronizing README.md..."
	@$(BIN_DIR)/$(BINARY_NAME) sync-readme

help:
	@echo "Available targets:"
	@echo "  make build       - Build the binary"
	@echo "  make test        - Run tests"
	@echo "  make clean       - Clean build artifacts"
	@echo "  make run ARGS=.. - Run the CLI with arguments"
	@echo "  make schema      - Generate JSON schema"
	@echo "  make generate-docs - Generate documentation using docgen"