FROM quay.io/hummingbird/go:1.27.3-builder@sha256:0aa859706feccd6c9eb401eca788a66e4cfed632af679d1bfd4019f96a543924 AS build

# Version is injected at build time; the container has no usable .git to derive
# it from (see `make container`). Defaults to "dev" for plain `podman build`.
ARG VERSION=dev

RUN dnf install -y make git && dnf clean all

WORKDIR /app

COPY go.mod go.sum ./
RUN go mod download

COPY . .

RUN CGO_ENABLED=0 GOOS=linux make build VERSION="${VERSION}"

FROM quay.io/hummingbird/core-runtime:2.43@sha256:2931ed4dc1e126857ec207419c6c758bbe8f4dc6048d14d8a1dbc1582fdd6e5e

WORKDIR /app

COPY --from=build /app/forgejo-mcp .

ENTRYPOINT ["/app/forgejo-mcp"]
