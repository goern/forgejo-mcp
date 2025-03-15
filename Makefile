GO ?= go
EXECUTABLE := gitea-mcp

.PHONY: build
build:
	$(GO) build -v -ldflags '-s -w' -o $(EXECUTABLE)