FROM quay.io/hummingbird/go:1.27.0-builder@sha256:053da9c6e7e5234362b167163057f328b04aaf69a3baaf775315101e2fa5f52a AS build

# Version is injected at build time; the container has no usable .git to derive
# it from (see `make container`). Defaults to "dev" for plain `podman build`.
ARG VERSION=dev

RUN dnf install -y make git && dnf clean all

WORKDIR /app

COPY go.mod go.sum ./
RUN go mod download

COPY . .

RUN CGO_ENABLED=0 GOOS=linux make build VERSION="${VERSION}"

FROM quay.io/hummingbird/core-runtime:2.43@sha256:71993808c91eb67af437cbd08eb03e997b80c6ebb8a376693eb113165b837cec

WORKDIR /app

COPY --from=build /app/forgejo-mcp .

ENTRYPOINT ["/app/forgejo-mcp"]
