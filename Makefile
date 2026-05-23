.PHONY: all build test lint fmt vet clean help tidy

APP_NAME := mall
BUILD_DIR := build

all: lint vet build test

build:
	@mkdir -p $(BUILD_DIR)
	go build -o $(BUILD_DIR)/$(APP_NAME) ./...

test:
	go test -count=1 -race ./...

lint:
	@which golangci-lint > /dev/null 2>&1 || (echo "Installing golangci-lint..."; go install github.com/golangci/golangci-lint/cmd/golangci-lint@latest)
	golangci-lint run ./...

fmt:
	go fmt ./...

vet:
	go vet ./...

tidy:
	go mod tidy
	go mod verify

clean:
	rm -rf $(BUILD_DIR)

help:
	@echo "Usage: make <target>"
	@echo ""
	@echo "Targets:"
	@echo "  build   Build the Go binary"
	@echo "  test    Run all tests (with race detector)"
	@echo "  lint    Run golangci-lint"
	@echo "  fmt     Format Go source code"
	@echo "  vet     Run go vet"
	@echo "  tidy    Tidy and verify go.mod"
	@echo "  clean   Remove build artifacts"
	@echo "  all     Run lint, vet, build, test"
