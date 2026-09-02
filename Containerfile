FROM quay.io/hummingbird/go:1.27.0-builder@sha256:e288dfb383304040f940af3d9c9e6577a12aa2acf508de594f83179c891a1197 AS build

# Version is injected at build time; the container has no usable .git to derive
# it from (see `make container`). Defaults to "dev" for plain `podman build`.
ARG VERSION=dev

RUN dnf install -y make git && dnf clean all

WORKDIR /app

COPY go.mod go.sum ./
RUN go mod download

COPY . .

RUN CGO_ENABLED=0 GOOS=linux make build VERSION="${VERSION}"

FROM quay.io/hummingbird/core-runtime:2.43@sha256:89c1c9bdc4d746526143c9d23f24c56d05ddc0e9c8e6f919e62601437ef8ed61

WORKDIR /app

COPY --from=build /app/forgejo-mcp .

ENTRYPOINT ["/app/forgejo-mcp"]
