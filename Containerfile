FROM quay.io/hummingbird/go:1.27.3-builder@sha256:61757cb638948e51a4d8430cd2be09eb505270f01d7be62750cf0961b26dc47a AS build

# Version is injected at build time; the container has no usable .git to derive
# it from (see `make container`). Defaults to "dev" for plain `podman build`.
ARG VERSION=dev

RUN dnf install -y make git && dnf clean all

WORKDIR /app

COPY go.mod go.sum ./
RUN go mod download

COPY . .

RUN CGO_ENABLED=0 GOOS=linux make build VERSION="${VERSION}"

FROM quay.io/hummingbird/core-runtime:2.43@sha256:81280d7fd6afa7a3d2a6da2e90e545cf8e5d187c80a69b2a1c875fdcd731aaed

WORKDIR /app

COPY --from=build /app/forgejo-mcp .

ENTRYPOINT ["/app/forgejo-mcp"]
