.PHONY: all build clean format test test-installer vet verify release-arm64 release-amd64

GO ?= go
BIN_DIR := bin
VERSION ?= dev
LDFLAGS := -s -w -X main.version=$(VERSION)

all: build

build:
	@mkdir -p $(BIN_DIR)
	$(GO) build -trimpath -ldflags="$(LDFLAGS)" -o $(BIN_DIR)/stoat ./cmd/stoat

format:
	$(GO) fmt ./...

test:
	$(GO) test -race ./...

test-installer:
	bash scripts/install_test.sh

vet:
	$(GO) vet ./...

verify:
	./scripts/check.sh

release-arm64:
	@mkdir -p $(BIN_DIR)
	CGO_ENABLED=0 GOOS=darwin GOARCH=arm64 $(GO) build -trimpath -ldflags="$(LDFLAGS)" -o $(BIN_DIR)/stoat-darwin-arm64 ./cmd/stoat

release-amd64:
	@mkdir -p $(BIN_DIR)
	CGO_ENABLED=0 GOOS=darwin GOARCH=amd64 $(GO) build -trimpath -ldflags="$(LDFLAGS)" -o $(BIN_DIR)/stoat-darwin-amd64 ./cmd/stoat

clean:
	rm -f $(BIN_DIR)/stoat $(BIN_DIR)/stoat-darwin-arm64 $(BIN_DIR)/stoat-darwin-amd64
