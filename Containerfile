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

FROM quay.io/hummingbird/core-runtime:2.42@sha256:02ca768db83eda71f60e3dec80ec31438a78a38b376975390cd332a3658f7478

WORKDIR /app

COPY --from=build /app/forgejo-mcp .

ENTRYPOINT ["/app/forgejo-mcp"]
