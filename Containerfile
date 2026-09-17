FROM quay.io/hummingbird/go:1.27.1-builder@sha256:ee2e29ac0cd556c4bca4e2c6a7fc45cfeee4b7fc5b4cfeae22d274bd512ceeb8 AS build

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
