FROM quay.io/hummingbird/go:1.27.0-builder@sha256:d2d9202a7f8cc083e19b2fcd80410c8e416082bb5cf5f1636f048aa139d3678a AS build

# Version is injected at build time; the container has no usable .git to derive
# it from (see `make container`). Defaults to "dev" for plain `podman build`.
ARG VERSION=dev

RUN dnf install -y make git && dnf clean all

WORKDIR /app

COPY go.mod go.sum ./
RUN go mod download

COPY . .

RUN CGO_ENABLED=0 GOOS=linux make build VERSION="${VERSION}"

FROM quay.io/hummingbird/core-runtime:2.43@sha256:8f4f90ae5941225e09ef034c4476bbe7918d084b72aaf78e0e198c36e7117270

WORKDIR /app

COPY --from=build /app/forgejo-mcp .

ENTRYPOINT ["/app/forgejo-mcp"]
