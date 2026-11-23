FROM quay.io/hummingbird/go:1.27.4-builder@sha256:9f44138f0761dea4864aa070510c1d0e0792ecb7b0c995b213b1630e77dca6ef AS build

# Version is injected at build time; the container has no usable .git to derive
# it from (see `make container`). Defaults to "dev" for plain `podman build`.
ARG VERSION=dev

RUN dnf install -y make git && dnf clean all

WORKDIR /app

COPY go.mod go.sum ./
RUN go mod download

COPY . .

RUN CGO_ENABLED=0 GOOS=linux make build VERSION="${VERSION}"

FROM quay.io/hummingbird/core-runtime:2.43@sha256:33539044661f70521596967ee6581b12cc49c30c9f93bf5c52b622d2e9aa8b01

WORKDIR /app

COPY --from=build /app/forgejo-mcp .

ENTRYPOINT ["/app/forgejo-mcp"]
