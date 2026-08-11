FROM quay.io/hummingbird/go:1.26.5-builder@sha256:0ce86d3ab46dd9606fdc086be854b76e28b7938e4bd5d0c43bcce50950b6ae0b AS build

# Version is injected at build time; the container has no usable .git to derive
# it from (see `make container`). Defaults to "dev" for plain `podman build`.
ARG VERSION=dev

RUN dnf install -y make git && dnf clean all

WORKDIR /app

COPY go.mod go.sum ./
RUN go mod download

COPY . .

RUN CGO_ENABLED=0 GOOS=linux make build VERSION="${VERSION}"

FROM quay.io/hummingbird/core-runtime:2.43@sha256:d885d5c40a938c5f7179538fd415c0b5a057b7823d2bd3593c428764ca920e96

WORKDIR /app

COPY --from=build /app/forgejo-mcp .

ENTRYPOINT ["/app/forgejo-mcp"]
