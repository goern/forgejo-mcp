FROM quay.io/hummingbird/go:1.26.5-builder@sha256:aea9ce90cd385d552f4136b4e2794aa207d4704882446350b386b88d55e7c673 AS build

# Version is injected at build time; the container has no usable .git to derive
# it from (see `make container`). Defaults to "dev" for plain `podman build`.
ARG VERSION=dev

RUN dnf install -y make git && dnf clean all

WORKDIR /app

COPY go.mod go.sum ./
RUN go mod download

COPY . .

RUN CGO_ENABLED=0 GOOS=linux make build VERSION="${VERSION}"

FROM quay.io/hummingbird/core-runtime:2.42@sha256:809fd66403145e6e98be0f8cce601f4fd0a685ff9cd3d11795aa987250d15c42

WORKDIR /app

COPY --from=build /app/forgejo-mcp .

ENTRYPOINT ["/app/forgejo-mcp"]
