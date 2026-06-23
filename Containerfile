FROM quay.io/hummingbird/go:1.26.4-builder@sha256:86156522b1fe99fa28be880c3e86db9fdedbcf240643004d5090ce71be89de47 AS build

# Version is injected at build time; the container has no usable .git to derive
# it from (see `make container`). Defaults to "dev" for plain `podman build`.
ARG VERSION=dev

RUN dnf install -y make git && dnf clean all

WORKDIR /app

COPY go.mod go.sum ./
RUN go mod download

COPY . .

RUN CGO_ENABLED=0 GOOS=linux make build VERSION="${VERSION}"

FROM quay.io/hummingbird/core-runtime:2.42@sha256:b9bbc03ef0531e5ea1918d1ec259b8414ce369842aba378cbf768a572ee26782

WORKDIR /app

COPY --from=build /app/forgejo-mcp .

ENTRYPOINT ["/app/forgejo-mcp"]
