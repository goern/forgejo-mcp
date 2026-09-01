FROM quay.io/hummingbird/go:1.27.0-builder@sha256:b2068301e9b23de3fea815053e3045b5093fc531c765f39a3ea0fa64e936502e AS build

# Version is injected at build time; the container has no usable .git to derive
# it from (see `make container`). Defaults to "dev" for plain `podman build`.
ARG VERSION=dev

RUN dnf install -y make git && dnf clean all

WORKDIR /app

COPY go.mod go.sum ./
RUN go mod download

COPY . .

RUN CGO_ENABLED=0 GOOS=linux make build VERSION="${VERSION}"

FROM quay.io/hummingbird/core-runtime:2.43@sha256:116df88c7ed74eb70660867293f30e825b1284f9bbafa77bc32d3c24dc92da54

WORKDIR /app

COPY --from=build /app/forgejo-mcp .

ENTRYPOINT ["/app/forgejo-mcp"]
