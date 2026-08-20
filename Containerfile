FROM quay.io/hummingbird/go:1.27.0-builder@sha256:68af95f8ec89b3e69d9a047233f9c55bcdc36cfef1559f9d7468300d07a62f45 AS build

# Version is injected at build time; the container has no usable .git to derive
# it from (see `make container`). Defaults to "dev" for plain `podman build`.
ARG VERSION=dev

RUN dnf install -y make git && dnf clean all

WORKDIR /app

COPY go.mod go.sum ./
RUN go mod download

COPY . .

RUN CGO_ENABLED=0 GOOS=linux make build VERSION="${VERSION}"

FROM quay.io/hummingbird/core-runtime:2.43@sha256:71993808c91eb67af437cbd08eb03e997b80c6ebb8a376693eb113165b837cec

WORKDIR /app

COPY --from=build /app/forgejo-mcp .

ENTRYPOINT ["/app/forgejo-mcp"]
