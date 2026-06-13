FROM quay.io/hummingbird/go:1.26.4-builder@sha256:cc03ec271818fa288a22e3e2bf9da106264ab5d0830c034ee88e5d6b93402ef8 AS build

# Version is injected at build time; the container has no usable .git to derive
# it from (see `make container`). Defaults to "dev" for plain `podman build`.
ARG VERSION=dev

RUN dnf install -y make git && dnf clean all

WORKDIR /app

COPY go.mod go.sum ./
RUN go mod download

COPY . .

RUN CGO_ENABLED=0 GOOS=linux make build VERSION="${VERSION}"

FROM quay.io/hummingbird/core-runtime:2.42@sha256:0cded499d282cfa9e63d68055ec964211cea6cceda6e081238fbcad1e8d79747

WORKDIR /app

COPY --from=build /app/forgejo-mcp .

ENTRYPOINT ["/app/forgejo-mcp"]
