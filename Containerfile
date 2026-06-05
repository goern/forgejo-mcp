FROM quay.io/hummingbird/go:1.26.4-builder@sha256:cdef29626fcb59209c8a96b79f77fb49ab1bf353003875ac3386c8304896f585 AS build

# Version is injected at build time; the container has no usable .git to derive
# it from (see `make container`). Defaults to "dev" for plain `podman build`.
ARG VERSION=dev

RUN dnf install -y make git && dnf clean all

WORKDIR /app

COPY go.mod go.sum ./
RUN go mod download

COPY . .

RUN CGO_ENABLED=0 GOOS=linux make build VERSION="${VERSION}"

FROM quay.io/hummingbird/core-runtime:2.42@sha256:a71de5cc9a2f0d59a606993c43708ee700e5c7fbe997020cb9b31c7d12268b9d

WORKDIR /app

COPY --from=build /app/forgejo-mcp .

ENTRYPOINT ["/app/forgejo-mcp"]
