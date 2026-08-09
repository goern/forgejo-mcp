FROM quay.io/hummingbird/go:1.26.5-builder@sha256:f61609d53f7e2186f6d6ecb9772ef1bfc2f1244e29c11c4bb148e520eba2864d AS build

# Version is injected at build time; the container has no usable .git to derive
# it from (see `make container`). Defaults to "dev" for plain `podman build`.
ARG VERSION=dev

RUN dnf install -y make git && dnf clean all

WORKDIR /app

COPY go.mod go.sum ./
RUN go mod download

COPY . .

RUN CGO_ENABLED=0 GOOS=linux make build VERSION="${VERSION}"

FROM quay.io/hummingbird/core-runtime:2.43@sha256:a4e512a621c761d9fa98d097e1b7c0ccf5027f6f792274629638b61f72aa426e

WORKDIR /app

COPY --from=build /app/forgejo-mcp .

ENTRYPOINT ["/app/forgejo-mcp"]
