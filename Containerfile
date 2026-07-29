FROM quay.io/hummingbird/go:1.26.5-builder@sha256:e13883b4e56cc4bb1b2b4f2ed74162615ec37de8d18b2f4333119c236f2723a4 AS build

# Version is injected at build time; the container has no usable .git to derive
# it from (see `make container`). Defaults to "dev" for plain `podman build`.
ARG VERSION=dev

RUN dnf install -y make git && dnf clean all

WORKDIR /app

COPY go.mod go.sum ./
RUN go mod download

COPY . .

RUN CGO_ENABLED=0 GOOS=linux make build VERSION="${VERSION}"

FROM quay.io/hummingbird/core-runtime:2.43@sha256:974dec56b9fb3d6922ef585ed456d3cc705845002d1b45eeb3bf52314382a740

WORKDIR /app

COPY --from=build /app/forgejo-mcp .

ENTRYPOINT ["/app/forgejo-mcp"]
