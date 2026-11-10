FROM quay.io/hummingbird/go:1.27.3-builder@sha256:c7df57a7cf6902992f44b456cecc84c0b76bc1c19a60b7cb326238e5b91c5567 AS build

# Version is injected at build time; the container has no usable .git to derive
# it from (see `make container`). Defaults to "dev" for plain `podman build`.
ARG VERSION=dev

RUN dnf install -y make git && dnf clean all

WORKDIR /app

COPY go.mod go.sum ./
RUN go mod download

COPY . .

RUN CGO_ENABLED=0 GOOS=linux make build VERSION="${VERSION}"

FROM quay.io/hummingbird/core-runtime:2.43@sha256:ff1561e244760054f533762900d56172882c06cce90a4ad8d3e03d734ad3cfdf

WORKDIR /app

COPY --from=build /app/forgejo-mcp .

ENTRYPOINT ["/app/forgejo-mcp"]
