FROM quay.io/hummingbird/go:1.27.0-builder@sha256:d203a1fb284ad988bc860c003a7c77a65e2922f413e4f406cb855b77d7fd3e5d AS build

# Version is injected at build time; the container has no usable .git to derive
# it from (see `make container`). Defaults to "dev" for plain `podman build`.
ARG VERSION=dev

RUN dnf install -y make git && dnf clean all

WORKDIR /app

COPY go.mod go.sum ./
RUN go mod download

COPY . .

RUN CGO_ENABLED=0 GOOS=linux make build VERSION="${VERSION}"

FROM quay.io/hummingbird/core-runtime:2.43@sha256:c435c1e85e7036af150e1f74cd81f34cf05be7aaaab2de8f66f73ac15879283d

WORKDIR /app

COPY --from=build /app/forgejo-mcp .

ENTRYPOINT ["/app/forgejo-mcp"]
