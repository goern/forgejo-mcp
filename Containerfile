FROM quay.io/hummingbird/go:1.26.4-builder@sha256:535beba1410e978862c64782d5f9c596d89263312f1a5f868bd6dfa3ef09c8da AS build

# Version is injected at build time; the container has no usable .git to derive
# it from (see `make container`). Defaults to "dev" for plain `podman build`.
ARG VERSION=dev

RUN dnf install -y make git && dnf clean all

WORKDIR /app

COPY go.mod go.sum ./
RUN go mod download

COPY . .

RUN CGO_ENABLED=0 GOOS=linux make build VERSION="${VERSION}"

FROM quay.io/hummingbird/core-runtime:2.42@sha256:9c7c8552fee26f6ae292e5f959a1987ab141f6bad69be0e3063686c86dbaf1a4

WORKDIR /app

COPY --from=build /app/forgejo-mcp .

ENTRYPOINT ["/app/forgejo-mcp"]
