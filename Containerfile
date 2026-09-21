FROM quay.io/hummingbird/go:1.27.1-builder@sha256:ed97ed1cd597207baec4a3c9f8caecc7e77be3a19ee1426beece09656821a8f3 AS build

# Version is injected at build time; the container has no usable .git to derive
# it from (see `make container`). Defaults to "dev" for plain `podman build`.
ARG VERSION=dev

RUN dnf install -y make git && dnf clean all

WORKDIR /app

COPY go.mod go.sum ./
RUN go mod download

COPY . .

RUN CGO_ENABLED=0 GOOS=linux make build VERSION="${VERSION}"

FROM quay.io/hummingbird/core-runtime:2.43@sha256:9671a7668f2a0a041d90174dec5cd15eba7ad34227363630d3fea11efa986a41

WORKDIR /app

COPY --from=build /app/forgejo-mcp .

ENTRYPOINT ["/app/forgejo-mcp"]
