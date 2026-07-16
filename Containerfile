FROM quay.io/hummingbird/go:1.26.5-builder@sha256:e84c6971fb7ee6709fcb15652ba920f03e0563a239dd235ab5b0e494c7e6985e AS build

# Version is injected at build time; the container has no usable .git to derive
# it from (see `make container`). Defaults to "dev" for plain `podman build`.
ARG VERSION=dev

RUN dnf install -y make git && dnf clean all

WORKDIR /app

COPY go.mod go.sum ./
RUN go mod download

COPY . .

RUN CGO_ENABLED=0 GOOS=linux make build VERSION="${VERSION}"

FROM quay.io/hummingbird/core-runtime:2.43@sha256:ff4c6de2e0bfc0d1cd771e8fad858ccbeff5dec01c55d18124f56653ee93eb79

WORKDIR /app

COPY --from=build /app/forgejo-mcp .

ENTRYPOINT ["/app/forgejo-mcp"]
