FROM quay.io/hummingbird/go:1.27.2-builder@sha256:c928ea1c9b29e092cb679614a7a0c114f6a614a6c808772d419db5330037d494 AS build

# Version is injected at build time; the container has no usable .git to derive
# it from (see `make container`). Defaults to "dev" for plain `podman build`.
ARG VERSION=dev

RUN dnf install -y make git && dnf clean all

WORKDIR /app

COPY go.mod go.sum ./
RUN go mod download

COPY . .

RUN CGO_ENABLED=0 GOOS=linux make build VERSION="${VERSION}"

FROM quay.io/hummingbird/core-runtime:2.43@sha256:6cb7b24518df0a9505f60a2a5d90431d5f727d9f4885bcb79cc9360a86f142bf

WORKDIR /app

COPY --from=build /app/forgejo-mcp .

ENTRYPOINT ["/app/forgejo-mcp"]
