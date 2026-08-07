FROM quay.io/hummingbird/go:1.26.5-builder@sha256:c7c22dd96cd2f542e262f22f42601a9ae54f5b9c098dad2f1b7d598f7b9dcebb AS build

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
