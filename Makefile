BINARY      := prometheus-inseego-exporter
# The version comes from version.txt; it is embedded in the binary and used for the RPM.
VERSION     := $(shell cat version.txt)
RELEASE     ?= 1
BUILD_DIR   := build
DIST_DIR    := dist

# The RPM targets Linux; override RPM_GOARCH for other architectures (e.g. arm64).
RPM_GOARCH  ?= amd64
NFPM        := go run github.com/goreleaser/nfpm/v2/cmd/nfpm@v2.48.0

# Commit hash (with -dirty for uncommitted changes) and UTC build date, embedded via -X.
COMMIT      := $(shell git rev-parse --short=12 HEAD 2>/dev/null || echo unknown)$(shell git diff --quiet HEAD 2>/dev/null || echo -dirty)
BUILD_DATE  := $(shell date -u +%Y-%m-%dT%H:%M:%SZ)

GO_LDFLAGS  := -s -w -X main.commit=$(COMMIT) -X main.buildDate=$(BUILD_DATE)

.PHONY: build build-rpm clean

## build: build the exporter binary for the host platform
build:
	go build -trimpath -ldflags '$(GO_LDFLAGS)' -o $(BUILD_DIR)/$(BINARY) .

## build-rpm: cross-compile for linux/$(RPM_GOARCH) and package it as an RPM
build-rpm:
	CGO_ENABLED=0 GOOS=linux GOARCH=$(RPM_GOARCH) \
		go build -trimpath -ldflags '$(GO_LDFLAGS)' -o $(BUILD_DIR)/rpm/$(BINARY) .
	mkdir -p $(DIST_DIR)
	VERSION=$(VERSION) RELEASE=$(RELEASE) GOARCH=$(RPM_GOARCH) \
		$(NFPM) package --config nfpm.yaml --packager rpm --target $(DIST_DIR)/

clean:
	rm -rf $(BUILD_DIR) $(DIST_DIR)
