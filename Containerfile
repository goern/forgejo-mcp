FROM quay.io/hummingbird/go:1.26.5-builder@sha256:1fed154709b959145e2e8396c3a20a94e8921777e3d854fd91c05e1ec3466be0 AS build

# Version is injected at build time; the container has no usable .git to derive
# it from (see `make container`). Defaults to "dev" for plain `podman build`.
ARG VERSION=dev

RUN dnf install -y make git && dnf clean all

WORKDIR /app

COPY go.mod go.sum ./
RUN go mod download

COPY . .

RUN CGO_ENABLED=0 GOOS=linux make build VERSION="${VERSION}"

FROM quay.io/hummingbird/core-runtime:2.42@sha256:ed1c759f20fd9d4c7b540f5713e54ddc710ee57fc8b6d0108e91e2140319baa0

WORKDIR /app

COPY --from=build /app/forgejo-mcp .

ENTRYPOINT ["/app/forgejo-mcp"]
