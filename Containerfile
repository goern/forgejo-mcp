FROM quay.io/hummingbird/go:1.26.5-builder@sha256:ffd93845c81492813037a25b3ce56d8051434aaadd3290cb381e33df99f0f177 AS build

# Version is injected at build time; the container has no usable .git to derive
# it from (see `make container`). Defaults to "dev" for plain `podman build`.
ARG VERSION=dev

RUN dnf install -y make git && dnf clean all

WORKDIR /app

COPY go.mod go.sum ./
RUN go mod download

COPY . .

RUN CGO_ENABLED=0 GOOS=linux make build VERSION="${VERSION}"

FROM quay.io/hummingbird/core-runtime:2.43@sha256:a68f46712d14424b5d5991336eae822f33d7d721aef3a77d1c7289645410ce47

WORKDIR /app

COPY --from=build /app/forgejo-mcp .

ENTRYPOINT ["/app/forgejo-mcp"]
