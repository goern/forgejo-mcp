FROM quay.io/hummingbird/go:1.27.0-builder@sha256:b3ffe05e77225c95570d5e533aee52504b0d9205ee4e7bc186c3c8e1abffe3e4 AS build

# Version is injected at build time; the container has no usable .git to derive
# it from (see `make container`). Defaults to "dev" for plain `podman build`.
ARG VERSION=dev

RUN dnf install -y make git && dnf clean all

WORKDIR /app

COPY go.mod go.sum ./
RUN go mod download

COPY . .

RUN CGO_ENABLED=0 GOOS=linux make build VERSION="${VERSION}"

FROM quay.io/hummingbird/core-runtime:2.43@sha256:89c1c9bdc4d746526143c9d23f24c56d05ddc0e9c8e6f919e62601437ef8ed61

WORKDIR /app

COPY --from=build /app/forgejo-mcp .

ENTRYPOINT ["/app/forgejo-mcp"]
