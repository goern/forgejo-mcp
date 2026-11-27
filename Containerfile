FROM quay.io/hummingbird/go:1.27.4-builder@sha256:411b34db024ce2aa69eb30dc1332506ba4e43d8d3e079a958ab0d72df827cd51 AS build

# Version is injected at build time; the container has no usable .git to derive
# it from (see `make container`). Defaults to "dev" for plain `podman build`.
ARG VERSION=dev

RUN dnf install -y make git && dnf clean all

WORKDIR /app

COPY go.mod go.sum ./
RUN go mod download

COPY . .

RUN CGO_ENABLED=0 GOOS=linux make build VERSION="${VERSION}"

FROM quay.io/hummingbird/core-runtime:2.43@sha256:fe49c1e94db2743f99ea08d7bec7b7326595130463f76dd32308e6e122361149

WORKDIR /app

COPY --from=build /app/forgejo-mcp .

ENTRYPOINT ["/app/forgejo-mcp"]
