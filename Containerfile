FROM quay.io/hummingbird/go:1.26.5-builder@sha256:1c1f746dd867fed8e979211e8ea474a38f1dbec87abb4e3ac41ad0c3d228f7f0 AS build

# Version is injected at build time; the container has no usable .git to derive
# it from (see `make container`). Defaults to "dev" for plain `podman build`.
ARG VERSION=dev

RUN dnf install -y make git && dnf clean all

WORKDIR /app

COPY go.mod go.sum ./
RUN go mod download

COPY . .

RUN CGO_ENABLED=0 GOOS=linux make build VERSION="${VERSION}"

FROM quay.io/hummingbird/core-runtime:2.43@sha256:8792ecb75763b6a2f783e048722d87bcf41b9479dafe4c6d7f275781b91e9196

WORKDIR /app

COPY --from=build /app/forgejo-mcp .

ENTRYPOINT ["/app/forgejo-mcp"]
