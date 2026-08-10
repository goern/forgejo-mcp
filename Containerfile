FROM quay.io/hummingbird/go:1.26.5-builder@sha256:cc410e4909b11b0c952af808986d8f1ed503f28d74cf9b0a99492cfb7cf98efe AS build

# Version is injected at build time; the container has no usable .git to derive
# it from (see `make container`). Defaults to "dev" for plain `podman build`.
ARG VERSION=dev

RUN dnf install -y make git && dnf clean all

WORKDIR /app

COPY go.mod go.sum ./
RUN go mod download

COPY . .

RUN CGO_ENABLED=0 GOOS=linux make build VERSION="${VERSION}"

FROM quay.io/hummingbird/core-runtime:2.43@sha256:eedfa678a1f6d28a30c3923a676097e48dde5d5d71738c29314497167bc0065c

WORKDIR /app

COPY --from=build /app/forgejo-mcp .

ENTRYPOINT ["/app/forgejo-mcp"]
