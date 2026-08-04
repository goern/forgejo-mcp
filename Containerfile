FROM quay.io/hummingbird/go:1.26.5-builder@sha256:b163b5484b7b776b285d851414039ed21fc358f4249ecb2901e0f1ba2db2e9b6 AS build

# Version is injected at build time; the container has no usable .git to derive
# it from (see `make container`). Defaults to "dev" for plain `podman build`.
ARG VERSION=dev

RUN dnf install -y make git && dnf clean all

WORKDIR /app

COPY go.mod go.sum ./
RUN go mod download

COPY . .

RUN CGO_ENABLED=0 GOOS=linux make build VERSION="${VERSION}"

FROM quay.io/hummingbird/core-runtime:2.43@sha256:58b11e40825f2be4cb596b66009fa79573fd6f510278f9e766a8b09e343a91b2

WORKDIR /app

COPY --from=build /app/forgejo-mcp .

ENTRYPOINT ["/app/forgejo-mcp"]
