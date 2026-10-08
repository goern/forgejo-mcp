FROM quay.io/hummingbird/go:1.27.1-builder@sha256:1a46b1315253b3b5c1921b2796ed72467a5dbf6b54f6ff1778b4c4ab203dd359 AS build

# Version is injected at build time; the container has no usable .git to derive
# it from (see `make container`). Defaults to "dev" for plain `podman build`.
ARG VERSION=dev

RUN dnf install -y make git && dnf clean all

WORKDIR /app

COPY go.mod go.sum ./
RUN go mod download

COPY . .

RUN CGO_ENABLED=0 GOOS=linux make build VERSION="${VERSION}"

FROM quay.io/hummingbird/core-runtime:2.43@sha256:06f90824df42c6d2dae1ff7b457b3d24e0e0901554dbf2f69d3fbd0886cd0635

WORKDIR /app

COPY --from=build /app/forgejo-mcp .

ENTRYPOINT ["/app/forgejo-mcp"]
