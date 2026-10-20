FROM quay.io/hummingbird/go:1.27.3-builder@sha256:2ccddb0a3819e734b9cfd598d134ce0db0a2076aebdb60da18fe65ba1983fb1d AS build

# Version is injected at build time; the container has no usable .git to derive
# it from (see `make container`). Defaults to "dev" for plain `podman build`.
ARG VERSION=dev

RUN dnf install -y make git && dnf clean all

WORKDIR /app

COPY go.mod go.sum ./
RUN go mod download

COPY . .

RUN CGO_ENABLED=0 GOOS=linux make build VERSION="${VERSION}"

FROM quay.io/hummingbird/core-runtime:2.43@sha256:9ba6530eb7cbfc625fb747df77865802f5e6249cdfc88d500bdcdb2690f6e2b8

WORKDIR /app

COPY --from=build /app/forgejo-mcp .

ENTRYPOINT ["/app/forgejo-mcp"]
