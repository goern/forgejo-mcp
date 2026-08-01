FROM quay.io/hummingbird/go:1.26.5-builder@sha256:c2c7b0e204a331eb392a60cc1fcdeade8cf881f682f8370e5668c5c70d723328 AS build

# Version is injected at build time; the container has no usable .git to derive
# it from (see `make container`). Defaults to "dev" for plain `podman build`.
ARG VERSION=dev

RUN dnf install -y make git && dnf clean all

WORKDIR /app

COPY go.mod go.sum ./
RUN go mod download

COPY . .

RUN CGO_ENABLED=0 GOOS=linux make build VERSION="${VERSION}"

FROM quay.io/hummingbird/core-runtime:2.43@sha256:80bd4bd4e74c8ab4e372cf355b335dbd30d91e9e5669822283299e65c64788e2

WORKDIR /app

COPY --from=build /app/forgejo-mcp .

ENTRYPOINT ["/app/forgejo-mcp"]
