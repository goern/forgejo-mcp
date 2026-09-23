FROM quay.io/hummingbird/go:1.27.1-builder@sha256:70f77fe1547262cb67169ed5bb414ef4edb4579dab58e3519704865baba10789 AS build

# Version is injected at build time; the container has no usable .git to derive
# it from (see `make container`). Defaults to "dev" for plain `podman build`.
ARG VERSION=dev

RUN dnf install -y make git && dnf clean all

WORKDIR /app

COPY go.mod go.sum ./
RUN go mod download

COPY . .

RUN CGO_ENABLED=0 GOOS=linux make build VERSION="${VERSION}"

FROM quay.io/hummingbird/core-runtime:2.43@sha256:b3e95d0d4307863ecaa8e682e855e719c26bcdffd566b8ef292909009d19428c

WORKDIR /app

COPY --from=build /app/forgejo-mcp .

ENTRYPOINT ["/app/forgejo-mcp"]
