FROM quay.io/hummingbird/go:1.26.5-builder@sha256:1864beb31e4942b9fb576601843ee9c86d0659e3bc70b110ad1ab0dc1c3c0d8a AS build

# Version is injected at build time; the container has no usable .git to derive
# it from (see `make container`). Defaults to "dev" for plain `podman build`.
ARG VERSION=dev

RUN dnf install -y make git && dnf clean all

WORKDIR /app

COPY go.mod go.sum ./
RUN go mod download

COPY . .

RUN CGO_ENABLED=0 GOOS=linux make build VERSION="${VERSION}"

FROM quay.io/hummingbird/core-runtime:2.43@sha256:ef510aa7911285fb9467af1a92cec48e3c660511ea4687cbc220249fab6a245d

WORKDIR /app

COPY --from=build /app/forgejo-mcp .

ENTRYPOINT ["/app/forgejo-mcp"]
