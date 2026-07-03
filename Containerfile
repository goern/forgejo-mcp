FROM quay.io/hummingbird/go:1.26.4-builder@sha256:350605674dc153f0856195023522998e716b25c2d338274f91ea8f49b43a9d6c AS build

# Version is injected at build time; the container has no usable .git to derive
# it from (see `make container`). Defaults to "dev" for plain `podman build`.
ARG VERSION=dev

RUN dnf install -y make git && dnf clean all

WORKDIR /app

COPY go.mod go.sum ./
RUN go mod download

COPY . .

RUN CGO_ENABLED=0 GOOS=linux make build VERSION="${VERSION}"

FROM quay.io/hummingbird/core-runtime:2.42@sha256:f2f612f57ac9387a403890d5f081add08feb3fe96e7b4fe66231dd5142dbdb8a

WORKDIR /app

COPY --from=build /app/forgejo-mcp .

ENTRYPOINT ["/app/forgejo-mcp"]
