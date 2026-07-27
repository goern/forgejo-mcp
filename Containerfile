FROM quay.io/hummingbird/go:1.26.5-builder@sha256:2d3c2eacf9a34161fc130ce100cf1cf36da1a0021add3680afbb703c242fdf4c AS build

# Version is injected at build time; the container has no usable .git to derive
# it from (see `make container`). Defaults to "dev" for plain `podman build`.
ARG VERSION=dev

RUN dnf install -y make git && dnf clean all

WORKDIR /app

COPY go.mod go.sum ./
RUN go mod download

COPY . .

RUN CGO_ENABLED=0 GOOS=linux make build VERSION="${VERSION}"

FROM quay.io/hummingbird/core-runtime:2.43@sha256:8e597a23a81b65132b7d64d827eb723b035324ec4565ab7aed442540ffbc0841

WORKDIR /app

COPY --from=build /app/forgejo-mcp .

ENTRYPOINT ["/app/forgejo-mcp"]
