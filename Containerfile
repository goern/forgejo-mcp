FROM quay.io/hummingbird/go:1.27.3-builder@sha256:7858cb85c26ddf3cb9d5f32b307d8fdaecbff1506bc2133e9c2454a4a3a3843d AS build

# Version is injected at build time; the container has no usable .git to derive
# it from (see `make container`). Defaults to "dev" for plain `podman build`.
ARG VERSION=dev

RUN dnf install -y make git && dnf clean all

WORKDIR /app

COPY go.mod go.sum ./
RUN go mod download

COPY . .

RUN CGO_ENABLED=0 GOOS=linux make build VERSION="${VERSION}"

FROM quay.io/hummingbird/core-runtime:2.43@sha256:d12d8cecc0120c6e50561140089265d41fd20f3138891bd52115da548cee5f31

WORKDIR /app

COPY --from=build /app/forgejo-mcp .

ENTRYPOINT ["/app/forgejo-mcp"]
