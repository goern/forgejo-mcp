FROM quay.io/hummingbird/go:1.27.4-builder@sha256:af2711142d9a9b34473581d45205db9e9aea4e3f46b649bbe20f41462f0b763e AS build

# Version is injected at build time; the container has no usable .git to derive
# it from (see `make container`). Defaults to "dev" for plain `podman build`.
ARG VERSION=dev

RUN dnf install -y make git && dnf clean all

WORKDIR /app

COPY go.mod go.sum ./
RUN go mod download

COPY . .

RUN CGO_ENABLED=0 GOOS=linux make build VERSION="${VERSION}"

FROM quay.io/hummingbird/core-runtime:2.43@sha256:fd005a4247ea89ab41c38768a42e651c99991b1df95d19cecbbc50c10cb3534b

WORKDIR /app

COPY --from=build /app/forgejo-mcp .

ENTRYPOINT ["/app/forgejo-mcp"]
