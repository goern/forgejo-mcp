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

FROM quay.io/hummingbird/core-runtime:2.43@sha256:a7366aeac22431636b322af8c518925fdc58908d1c2201ef67c844a131085b59

WORKDIR /app

COPY --from=build /app/forgejo-mcp .

ENTRYPOINT ["/app/forgejo-mcp"]
