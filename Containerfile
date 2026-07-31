FROM quay.io/hummingbird/go:1.26.5-builder@sha256:634c081c1cc0e9e79aaa7115ca3dc63fba04d28c3ef1a23f3bf500f37da00b22 AS build

# Version is injected at build time; the container has no usable .git to derive
# it from (see `make container`). Defaults to "dev" for plain `podman build`.
ARG VERSION=dev

RUN dnf install -y make git && dnf clean all

WORKDIR /app

COPY go.mod go.sum ./
RUN go mod download

COPY . .

RUN CGO_ENABLED=0 GOOS=linux make build VERSION="${VERSION}"

FROM quay.io/hummingbird/core-runtime:2.43@sha256:578a83d0e6716bc509c793987b924f8c4b49cec77c7e0f1b596038f558cf5b87

WORKDIR /app

COPY --from=build /app/forgejo-mcp .

ENTRYPOINT ["/app/forgejo-mcp"]
