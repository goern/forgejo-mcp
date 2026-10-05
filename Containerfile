FROM quay.io/hummingbird/go:1.27.1-builder@sha256:80e89be30fb1365ea851352eab8f071614d877d74c9f00e2159866c2b7c5bb23 AS build

# Version is injected at build time; the container has no usable .git to derive
# it from (see `make container`). Defaults to "dev" for plain `podman build`.
ARG VERSION=dev

RUN dnf install -y make git && dnf clean all

WORKDIR /app

COPY go.mod go.sum ./
RUN go mod download

COPY . .

RUN CGO_ENABLED=0 GOOS=linux make build VERSION="${VERSION}"

FROM quay.io/hummingbird/core-runtime:2.43@sha256:dbe63cc0af0d272c897f5e136173c9670f991deede43867f59876066dd6c4e5b

WORKDIR /app

COPY --from=build /app/forgejo-mcp .

ENTRYPOINT ["/app/forgejo-mcp"]
