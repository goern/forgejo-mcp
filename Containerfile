FROM quay.io/hummingbird/go:1.26.4-builder@sha256:4b5537d26ce31ced58deeb599f1990f5b55a533489e58cf6b66918388f250608 AS build

# Version is injected at build time; the container has no usable .git to derive
# it from (see `make container`). Defaults to "dev" for plain `podman build`.
ARG VERSION=dev

RUN dnf install -y make git && dnf clean all

WORKDIR /app

COPY go.mod go.sum ./
RUN go mod download

COPY . .

RUN CGO_ENABLED=0 GOOS=linux make build VERSION="${VERSION}"

FROM quay.io/hummingbird/core-runtime:2.42@sha256:47c4393878ee5848f91d9538dbe742b8cd04da6d1db80286c293460eeb5b1a6c

WORKDIR /app

COPY --from=build /app/forgejo-mcp .

ENTRYPOINT ["/app/forgejo-mcp"]
