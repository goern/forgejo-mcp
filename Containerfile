FROM quay.io/hummingbird/go:1.27.0-builder@sha256:3849756048007dcda054804de607da20262da27d2d9a2498c2cc3222ecd4d6b9 AS build

# Version is injected at build time; the container has no usable .git to derive
# it from (see `make container`). Defaults to "dev" for plain `podman build`.
ARG VERSION=dev

RUN dnf install -y make git && dnf clean all

WORKDIR /app

COPY go.mod go.sum ./
RUN go mod download

COPY . .

RUN CGO_ENABLED=0 GOOS=linux make build VERSION="${VERSION}"

FROM quay.io/hummingbird/core-runtime:2.43@sha256:fa72c318cd10f62b5616f396df0493ad8531adf0841aaf7b5d89c323712cfa11

WORKDIR /app

COPY --from=build /app/forgejo-mcp .

ENTRYPOINT ["/app/forgejo-mcp"]
