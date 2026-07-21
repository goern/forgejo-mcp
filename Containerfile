FROM quay.io/hummingbird/go:1.26.5-builder@sha256:eaad4f8e6aab2543848d3d5fb9a62836e4fc0be70b9df9d1134af318279fc2f3 AS build

# Version is injected at build time; the container has no usable .git to derive
# it from (see `make container`). Defaults to "dev" for plain `podman build`.
ARG VERSION=dev

RUN dnf install -y make git && dnf clean all

WORKDIR /app

COPY go.mod go.sum ./
RUN go mod download

COPY . .

RUN CGO_ENABLED=0 GOOS=linux make build VERSION="${VERSION}"

FROM quay.io/hummingbird/core-runtime:2.43@sha256:6ed1fc644c70a3461dd7e9fe8c488e7d1cc978f89e6dc4257036cafbfb2f6825

WORKDIR /app

COPY --from=build /app/forgejo-mcp .

ENTRYPOINT ["/app/forgejo-mcp"]
