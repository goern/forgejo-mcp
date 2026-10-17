FROM quay.io/hummingbird/go:1.27.2-builder@sha256:af6cb1d7c460cf5dfee4daee3aa4915b1e42b3375ec8aa93940576062b94629f AS build

# Version is injected at build time; the container has no usable .git to derive
# it from (see `make container`). Defaults to "dev" for plain `podman build`.
ARG VERSION=dev

RUN dnf install -y make git && dnf clean all

WORKDIR /app

COPY go.mod go.sum ./
RUN go mod download

COPY . .

RUN CGO_ENABLED=0 GOOS=linux make build VERSION="${VERSION}"

FROM quay.io/hummingbird/core-runtime:2.43@sha256:dde95dd9acc448a70565d2cd3d0bc384f8c2640bfd1dece6c89a6f0193e2b7fc

WORKDIR /app

COPY --from=build /app/forgejo-mcp .

ENTRYPOINT ["/app/forgejo-mcp"]
