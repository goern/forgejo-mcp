FROM quay.io/hummingbird/go:1.27.4-builder@sha256:d13bab862a001b80130e837be57ae574b24ba8d8b31f178074c7c08877f63967 AS build

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
