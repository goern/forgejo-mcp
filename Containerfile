FROM quay.io/hummingbird/go:1.27.2-builder@sha256:d1b10273947fb40ecf7ecfc5ed2ada8214b3bea2803bb5bc408fb823c08acac2 AS build

# Version is injected at build time; the container has no usable .git to derive
# it from (see `make container`). Defaults to "dev" for plain `podman build`.
ARG VERSION=dev

RUN dnf install -y make git && dnf clean all

WORKDIR /app

COPY go.mod go.sum ./
RUN go mod download

COPY . .

RUN CGO_ENABLED=0 GOOS=linux make build VERSION="${VERSION}"

FROM quay.io/hummingbird/core-runtime:2.43@sha256:4730fe5f23bec7eb86b9736bc1458d58862372b1d1555ca9da77d21d4fffea17

WORKDIR /app

COPY --from=build /app/forgejo-mcp .

ENTRYPOINT ["/app/forgejo-mcp"]
