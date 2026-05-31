GO ?= go
EXECUTABLE := forgejo-mcp
COMMIT ?= $(shell git rev-parse --short HEAD)
VERSION ?= $(shell v=$$(git describe --tags --exact-match 2>/dev/null) && echo "$$v" | sed 's/^v//' || echo "$$(git describe --tags --abbrev=0 2>/dev/null | sed 's/^v//')-dev+$(COMMIT)")
LDFLAGS := -X "main.Version=$(VERSION)"

.DEFAULT_GOAL := help

## help: show available make targets
.PHONY: help
help:
	@echo "Usage: make <target>"
	@echo ""
	@echo "Targets:"
	@awk 'BEGIN {FS=":"} /^## / {sub(/^## /,""); printf "  \033[36m%-12s\033[0m %s\n", $$1, $$2}' $(MAKEFILE_LIST)

## build: build the forgejo-mcp binary
.PHONY: build
build:
	$(GO) build -v -ldflags '-s -w $(LDFLAGS)' -o $(EXECUTABLE)

## vendor: tidy and verify module dependencies
.PHONY: vendor
vendor:
	@echo 'Tidying and verifying module dependencies...'
	go mod tidy
	go mod verify

## container: build container image using podman
IMAGE_NAME ?= forgejo-mcp
IMAGE_TAG ?= $(shell git describe --tags --abbrev=0 2>/dev/null | sed 's/^v//' || echo "latest")

.PHONY: container
container:
	podman build --build-arg VERSION=$(VERSION) -t $(IMAGE_NAME):$(IMAGE_TAG) -f Containerfile .

## mcpb: build the Claude Desktop Extension (.mcpb) for the host platform
GOOS ?= $(shell $(GO) env GOOS)
GOARCH ?= $(shell $(GO) env GOARCH)
MCPB_OUT ?= dist/forgejo-mcp_$(VERSION)_$(GOOS)_$(GOARCH).mcpb

.PHONY: mcpb
mcpb: build
	@command -v jq >/dev/null || { echo "jq is required" >&2; exit 1; }
	@command -v npx >/dev/null || { echo "npx (Node.js) is required" >&2; exit 1; }
	@mkdir -p dist extension/bin
	cp $(EXECUTABLE) extension/bin/forgejo-mcp
	chmod +x extension/bin/forgejo-mcp
	cp extension/manifest.json extension/manifest.json.orig
	@trap 'mv -f extension/manifest.json.orig extension/manifest.json 2>/dev/null || true' EXIT; \
		jq --arg v "$(VERSION)" '.version=$$v' extension/manifest.json > extension/manifest.json.tmp && \
		mv extension/manifest.json.tmp extension/manifest.json && \
		npx -y @anthropic-ai/mcpb pack extension/ "$(MCPB_OUT)"
	@echo "Built $(MCPB_OUT)"
