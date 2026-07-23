FROM quay.io/hummingbird/go:1.26.5-builder@sha256:5ce47e511d42d112aa998eab3dcf4ab8ccfceaebf557a32e49baf7e74532a698 AS build

# Version is injected at build time; the container has no usable .git to derive
# it from (see `make container`). Defaults to "dev" for plain `podman build`.
ARG VERSION=dev

RUN dnf install -y make git && dnf clean all

WORKDIR /app

COPY go.mod go.sum ./
RUN go mod download

COPY . .

RUN CGO_ENABLED=0 GOOS=linux make build VERSION="${VERSION}"

FROM quay.io/hummingbird/core-runtime:2.43@sha256:dda9f3b87503795f20c5821a496f7a0228642da31a9481c9793b37d8a6463cff

WORKDIR /app

COPY --from=build /app/forgejo-mcp .

ENTRYPOINT ["/app/forgejo-mcp"]
