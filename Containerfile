FROM quay.io/hummingbird/go:1.26.5-builder@sha256:f5f37809c57abf661a63bf37824507b9ead65391d40976de85651348aedaae99 AS build

# Version is injected at build time; the container has no usable .git to derive
# it from (see `make container`). Defaults to "dev" for plain `podman build`.
ARG VERSION=dev

RUN dnf install -y make git && dnf clean all

WORKDIR /app

COPY go.mod go.sum ./
RUN go mod download

COPY . .

RUN CGO_ENABLED=0 GOOS=linux make build VERSION="${VERSION}"

FROM quay.io/hummingbird/core-runtime:2.43@sha256:dfbb233c247bd261bb45cb2e9d36bc29181325f27423b1749995d5a0b606acef

WORKDIR /app

COPY --from=build /app/forgejo-mcp .

ENTRYPOINT ["/app/forgejo-mcp"]
