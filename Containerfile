FROM quay.io/hummingbird/go:1.27.1-builder@sha256:2b7b566e0b90ec21598d7a6c3242ba09117f80ccd96c3c823f47eebc94bc99b8 AS build

# Version is injected at build time; the container has no usable .git to derive
# it from (see `make container`). Defaults to "dev" for plain `podman build`.
ARG VERSION=dev

RUN dnf install -y make git && dnf clean all

WORKDIR /app

COPY go.mod go.sum ./
RUN go mod download

COPY . .

RUN CGO_ENABLED=0 GOOS=linux make build VERSION="${VERSION}"

FROM quay.io/hummingbird/core-runtime:2.43@sha256:959fb8aa53d17839c62984605e36f184775f4664df4df8784f613209fdfe5d4a

WORKDIR /app

COPY --from=build /app/forgejo-mcp .

ENTRYPOINT ["/app/forgejo-mcp"]
