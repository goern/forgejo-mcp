FROM quay.io/hummingbird/go:1.27.3-builder@sha256:6d191e5250d5a9d1385a19cecd152d8a4a1687fa937940720c5e144c4a249957 AS build

# Version is injected at build time; the container has no usable .git to derive
# it from (see `make container`). Defaults to "dev" for plain `podman build`.
ARG VERSION=dev

RUN dnf install -y make git && dnf clean all

WORKDIR /app

COPY go.mod go.sum ./
RUN go mod download

COPY . .

RUN CGO_ENABLED=0 GOOS=linux make build VERSION="${VERSION}"

FROM quay.io/hummingbird/core-runtime:2.43@sha256:329ff8f46fe69f036b96c3463272aa778a6df6a4f7570f080fd248a414652b6e

WORKDIR /app

COPY --from=build /app/forgejo-mcp .

ENTRYPOINT ["/app/forgejo-mcp"]
