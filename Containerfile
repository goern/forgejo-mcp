FROM quay.io/hummingbird/go:1.26.5-builder@sha256:450f2624c86211e42c6b6619992556c2dc95101de4e441737a4254d9ed1cafd1 AS build

# Version is injected at build time; the container has no usable .git to derive
# it from (see `make container`). Defaults to "dev" for plain `podman build`.
ARG VERSION=dev

RUN dnf install -y make git && dnf clean all

WORKDIR /app

COPY go.mod go.sum ./
RUN go mod download

COPY . .

RUN CGO_ENABLED=0 GOOS=linux make build VERSION="${VERSION}"

FROM quay.io/hummingbird/core-runtime:2.43@sha256:9d7373fd0469f50a872da9fef7996381fb81bc4e23a0b08cbbd01599034f9f80

WORKDIR /app

COPY --from=build /app/forgejo-mcp .

ENTRYPOINT ["/app/forgejo-mcp"]
