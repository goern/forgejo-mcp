FROM quay.io/hummingbird/go:1.26.4-builder@sha256:251a0657abc7efb98b97b559afa719da3a48b975cba0c8a2c62e0e5e5264d7ad AS build

# Version is injected at build time; the container has no usable .git to derive
# it from (see `make container`). Defaults to "dev" for plain `podman build`.
ARG VERSION=dev

RUN dnf install -y make git && dnf clean all

WORKDIR /app

COPY go.mod go.sum ./
RUN go mod download

COPY . .

RUN CGO_ENABLED=0 GOOS=linux make build VERSION="${VERSION}"

FROM quay.io/hummingbird/core-runtime:2.42@sha256:3fadedf666f137f7d36a673fcf215307bf19bc56c12eb6a323674606eac3c5bf

WORKDIR /app

COPY --from=build /app/forgejo-mcp .

ENTRYPOINT ["/app/forgejo-mcp"]
