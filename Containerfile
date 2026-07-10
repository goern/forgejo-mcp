FROM quay.io/hummingbird/go:1.26.5-builder@sha256:37675d106cced23d6aeaf95f38f4662914c40cc703e2aa3f96184c34142e6f5c AS build

# Version is injected at build time; the container has no usable .git to derive
# it from (see `make container`). Defaults to "dev" for plain `podman build`.
ARG VERSION=dev

RUN dnf install -y make git && dnf clean all

WORKDIR /app

COPY go.mod go.sum ./
RUN go mod download

COPY . .

RUN CGO_ENABLED=0 GOOS=linux make build VERSION="${VERSION}"

FROM quay.io/hummingbird/core-runtime:2.42@sha256:8f628340892fc3b30a291b290f8522cbc8a906764764531c94aa1615fc309f94

WORKDIR /app

COPY --from=build /app/forgejo-mcp .

ENTRYPOINT ["/app/forgejo-mcp"]
