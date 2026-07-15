FROM quay.io/hummingbird/go:1.26.5-builder@sha256:f5f37809c57abf661a63bf37824507b9ead65391d40976de85651348aedaae99 AS build

# Version is injected at build time; the container has no usable .git to derive
# it from (see `make container`). Defaults to "dev" for plain `podman build`.
ARG VERSION=dev

RUN dnf install -y make git && dnf clean all

WORKDIR /app

COPY go.mod go.sum ./
RUN go mod download

COPY . .

RUN CGO_ENABLED=0 GOOS=linux make build VERSION="${VERSION}"

FROM quay.io/hummingbird/core-runtime:2.43@sha256:caa7dad1cefb58601e5789d826e46f56b37bc3543541c7bf4544a74ef57c5203

WORKDIR /app

COPY --from=build /app/forgejo-mcp .

ENTRYPOINT ["/app/forgejo-mcp"]
