GO ?= go
EXECUTABLE := gitea-mcp

.PHONY: build
build:
	$(GO) build -v -ldflags '-s -w' -o $(EXECUTABLE)

## air: install air for hot reload
.PHONY: air
air:
	@hash air > /dev/null 2>&1; if [ $$? -ne 0 ]; then \
		$(GO) install github.com/air-verse/air@latest; \
	fi

## dev: run the application with hot reload
.PHONY: dev
dev: air
	air --build.cmd "make build" --build.bin ./gitea-mcp

## vendor: tidy and verify module dependencies
.PHONY: vendor
vendor:
	@echo 'Tidying and verifying module dependencies...'
	go mod tidy
	go mod verify