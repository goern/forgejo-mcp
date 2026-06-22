FROM quay.io/hummingbird/go:1.26.4-builder@sha256:37676d96bda9a6ce92f9b5bb64cea2469502934444eea790091a000128a4299f AS build

# Version is injected at build time; the container has no usable .git to derive
# it from (see `make container`). Defaults to "dev" for plain `podman build`.
ARG VERSION=dev

RUN dnf install -y make git && dnf clean all

WORKDIR /app

COPY go.mod go.sum ./
RUN go mod download

COPY . .

RUN CGO_ENABLED=0 GOOS=linux make build VERSION="${VERSION}"

FROM quay.io/hummingbird/core-runtime:2.42@sha256:dcd72eaa2df901c4915e1eec915906c8787c64b9e4149b4211d4500fbbe71791

WORKDIR /app

COPY --from=build /app/forgejo-mcp .

ENTRYPOINT ["/app/forgejo-mcp"]
