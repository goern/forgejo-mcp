FROM quay.io/hummingbird/go:1.27.2-builder@sha256:11606492ec6393b21af3cfe1b98a82d7f91afec6f74cc1699af70a7a40234f5a AS build

# Version is injected at build time; the container has no usable .git to derive
# it from (see `make container`). Defaults to "dev" for plain `podman build`.
ARG VERSION=dev

RUN dnf install -y make git && dnf clean all

WORKDIR /app

COPY go.mod go.sum ./
RUN go mod download

COPY . .

RUN CGO_ENABLED=0 GOOS=linux make build VERSION="${VERSION}"

FROM quay.io/hummingbird/core-runtime:2.43@sha256:12ebc4493ccbb80fd47175c9c670ea590c4e7140a7c4858e038ac2df42be8ca8

WORKDIR /app

COPY --from=build /app/forgejo-mcp .

ENTRYPOINT ["/app/forgejo-mcp"]
