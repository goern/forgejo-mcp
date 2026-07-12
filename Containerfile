FROM quay.io/hummingbird/go:1.26.5-builder@sha256:c0955aeba1495dcb11392f60dbf43c46437286649bfcf94d23ca2635dd1eded2 AS build

# Version is injected at build time; the container has no usable .git to derive
# it from (see `make container`). Defaults to "dev" for plain `podman build`.
ARG VERSION=dev

RUN dnf install -y make git && dnf clean all

WORKDIR /app

COPY go.mod go.sum ./
RUN go mod download

COPY . .

RUN CGO_ENABLED=0 GOOS=linux make build VERSION="${VERSION}"

FROM quay.io/hummingbird/core-runtime:2.42@sha256:33d3baae2812b4a5e859a0cb8010369e227218c637ee5b8c8acb4768cd7644ff

WORKDIR /app

COPY --from=build /app/forgejo-mcp .

ENTRYPOINT ["/app/forgejo-mcp"]
