FROM quay.io/hummingbird/go:1.26.5-builder@sha256:c7231fca5d95a6a44cf168e200229440ab83ec87b15a2f867a0de44e03195966 AS build

# Version is injected at build time; the container has no usable .git to derive
# it from (see `make container`). Defaults to "dev" for plain `podman build`.
ARG VERSION=dev

RUN dnf install -y make git && dnf clean all

WORKDIR /app

COPY go.mod go.sum ./
RUN go mod download

COPY . .

RUN CGO_ENABLED=0 GOOS=linux make build VERSION="${VERSION}"

FROM quay.io/hummingbird/core-runtime:2.43@sha256:a9acec2a2ff60f41fdcb2047accd1b5dc2cee5599ed2f77ea2e0f01d9de1c258

WORKDIR /app

COPY --from=build /app/forgejo-mcp .

ENTRYPOINT ["/app/forgejo-mcp"]
