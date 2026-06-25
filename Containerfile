FROM quay.io/hummingbird/go:1.26.4-builder@sha256:d444a0ceb7d486319e4b15c87ad0ff3a90ef2fe37c01d71cc004649755738d97 AS build

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
