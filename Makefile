GO ?= go
EXECUTABLE := forgejo-mcp
COMMIT ?= $(shell git rev-parse --short HEAD)
VERSION ?= $(shell v=$$(git describe --tags --exact-match 2>/dev/null) && echo "$$v" | sed 's/^v//' || echo "$$(git describe --tags --abbrev=0 2>/dev/null | sed 's/^v//')-dev+$(COMMIT)")
LDFLAGS := -X "main.Version=$(VERSION)"

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
	podman build -t $(IMAGE_NAME):$(IMAGE_TAG) -f Containerfile .
