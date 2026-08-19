FROM quay.io/hummingbird/go:1.26.6-builder@sha256:4a8c3a14293364a1dff97915f225750327dd233b100d5ffc11576546e44ca7cf AS build

# Version is injected at build time; the container has no usable .git to derive
# it from (see `make container`). Defaults to "dev" for plain `podman build`.
ARG VERSION=dev

RUN dnf install -y make git && dnf clean all

WORKDIR /app

COPY go.mod go.sum ./
RUN go mod download

COPY . .

RUN CGO_ENABLED=0 GOOS=linux make build VERSION="${VERSION}"

FROM quay.io/hummingbird/core-runtime:2.43@sha256:1b171b70ec4cc99471bf4b70d3e338d9c703325a9c3dad6b9f69839d907db474

WORKDIR /app

COPY --from=build /app/forgejo-mcp .

ENTRYPOINT ["/app/forgejo-mcp"]
