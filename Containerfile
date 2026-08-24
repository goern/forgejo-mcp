FROM quay.io/hummingbird/go:1.27.0-builder@sha256:8e77444c488ec9af5a1c663dc09a0b75f9b62b8c2e75886450fb2eac220e4d44 AS build

# Version is injected at build time; the container has no usable .git to derive
# it from (see `make container`). Defaults to "dev" for plain `podman build`.
ARG VERSION=dev

RUN dnf install -y make git && dnf clean all

WORKDIR /app

COPY go.mod go.sum ./
RUN go mod download

COPY . .

RUN CGO_ENABLED=0 GOOS=linux make build VERSION="${VERSION}"

FROM quay.io/hummingbird/core-runtime:2.43@sha256:7ea0a84c21948360e62f51cf2f087e8a994f3da487d92b3f81536a1c13be63d8

WORKDIR /app

COPY --from=build /app/forgejo-mcp .

ENTRYPOINT ["/app/forgejo-mcp"]
