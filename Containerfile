FROM quay.io/hummingbird/go:1.26.4-builder@sha256:6e77e0ac351bdba90f23371277addd45b3361e906521961162762533f1f2266d AS build

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
