FROM quay.io/hummingbird/go:1.26.4-builder@sha256:5c775f946b8356d986a705c4e128006969010d69d6113947c2965de379f03236 AS build

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
