FROM quay.io/hummingbird/go:1.26.5-builder@sha256:4161f6f50e00aeecaa41677ef5c3cc4490e80460b546da333938ed1d7f730431 AS build

# Version is injected at build time; the container has no usable .git to derive
# it from (see `make container`). Defaults to "dev" for plain `podman build`.
ARG VERSION=dev

RUN dnf install -y make git && dnf clean all

WORKDIR /app

COPY go.mod go.sum ./
RUN go mod download

COPY . .

RUN CGO_ENABLED=0 GOOS=linux make build VERSION="${VERSION}"

FROM quay.io/hummingbird/core-runtime:2.42@sha256:bc61e282b2bc85632330c5d75edf04237f7af069b2a7b15143afd42860b559d9

WORKDIR /app

COPY --from=build /app/forgejo-mcp .

ENTRYPOINT ["/app/forgejo-mcp"]
