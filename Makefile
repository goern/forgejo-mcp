GO ?= go
EXECUTABLE := forgejo-mcp
VERSION ?= $(shell git describe --tags --exact-match 2>/dev/null | sed 's/^v//' || git describe --tags --always | sed 's/^v//' | sed 's/-.*/+dev/')
COMMIT ?= $(shell git rev-parse --short HEAD)
LDFLAGS := -X "main.Version=$(VERSION)" -X "main.Commit=$(COMMIT)"

.PHONY: build
build:
	$(GO) build -v -ldflags '-s -w $(LDFLAGS)' -o $(EXECUTABLE)

## vendor: tidy and verify module dependencies
.PHONY: vendor
vendor:
	@echo 'Tidying and verifying module dependencies...'
	go mod tidy
	go mod verify
