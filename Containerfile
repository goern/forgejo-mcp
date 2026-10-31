FROM quay.io/hummingbird/go:1.27.3-builder@sha256:f70021e6b5bead1e6c9c8bfe91a143a0651f36b0a0a01c19317570ddf48bec61 AS build

# Version is injected at build time; the container has no usable .git to derive
# it from (see `make container`). Defaults to "dev" for plain `podman build`.
ARG VERSION=dev

RUN dnf install -y make git && dnf clean all

WORKDIR /app

COPY go.mod go.sum ./
RUN go mod download

COPY . .

RUN CGO_ENABLED=0 GOOS=linux make build VERSION="${VERSION}"

FROM quay.io/hummingbird/core-runtime:2.43@sha256:4163796d8b7f28021178093d31cb97c05336fd3f3a4d7006603a7fef29e997e7

WORKDIR /app

COPY --from=build /app/forgejo-mcp .

ENTRYPOINT ["/app/forgejo-mcp"]
