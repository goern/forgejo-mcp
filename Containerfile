FROM quay.io/hummingbird/go:1.27.0-builder@sha256:2d3bc7294e35ee9cbe5cdc31ee5ca9caa2cff664967805f46d32b64b3d1499fe AS build

# Version is injected at build time; the container has no usable .git to derive
# it from (see `make container`). Defaults to "dev" for plain `podman build`.
ARG VERSION=dev

RUN dnf install -y make git && dnf clean all

WORKDIR /app

COPY go.mod go.sum ./
RUN go mod download

COPY . .

RUN CGO_ENABLED=0 GOOS=linux make build VERSION="${VERSION}"

FROM quay.io/hummingbird/core-runtime:2.43@sha256:57621f2c9b2e9b7a5016f799e41971d5f4c24e74ba34afddde07aad91950f670

WORKDIR /app

COPY --from=build /app/forgejo-mcp .

ENTRYPOINT ["/app/forgejo-mcp"]
