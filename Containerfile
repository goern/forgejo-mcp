FROM quay.io/hummingbird/go:1.26.6-builder@sha256:4a8c3a14293364a1dff97915f225750327dd233b100d5ffc11576546e44ca7cf AS build

# Version is injected at build time; the container has no usable .git to derive
# it from (see `make container`). Defaults to "dev" for plain `podman build`.
ARG VERSION=dev

RUN dnf install -y make git && dnf clean all

WORKDIR /app

COPY go.mod go.sum ./
RUN go mod download

COPY . .

RUN CGO_ENABLED=0 GOOS=linux make build VERSION="${VERSION}"

FROM quay.io/hummingbird/core-runtime:2.43@sha256:048d54fc66bde5a64e2dd82d71cce1755eb090e18c2c1803b81c9ad30c0f9dca

WORKDIR /app

COPY --from=build /app/forgejo-mcp .

ENTRYPOINT ["/app/forgejo-mcp"]
