FROM quay.io/hummingbird/go:1.26.5-builder@sha256:38f1f53f2ede564c0821cc5df2628b4134e5fb7f55f2123c68d331e384a80a02 AS build

# Version is injected at build time; the container has no usable .git to derive
# it from (see `make container`). Defaults to "dev" for plain `podman build`.
ARG VERSION=dev

RUN dnf install -y make git && dnf clean all

WORKDIR /app

COPY go.mod go.sum ./
RUN go mod download

COPY . .

RUN CGO_ENABLED=0 GOOS=linux make build VERSION="${VERSION}"

FROM quay.io/hummingbird/core-runtime:2.43@sha256:ff4c6de2e0bfc0d1cd771e8fad858ccbeff5dec01c55d18124f56653ee93eb79

WORKDIR /app

COPY --from=build /app/forgejo-mcp .

ENTRYPOINT ["/app/forgejo-mcp"]
