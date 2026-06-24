FROM quay.io/hummingbird/go:1.26.4-builder@sha256:9508bfe1ba2fe567e3ea0edd5eba703d48cb135a181a45f73825611394d7f781 AS build

# Version is injected at build time; the container has no usable .git to derive
# it from (see `make container`). Defaults to "dev" for plain `podman build`.
ARG VERSION=dev

RUN dnf install -y make git && dnf clean all

WORKDIR /app

COPY go.mod go.sum ./
RUN go mod download

COPY . .

RUN CGO_ENABLED=0 GOOS=linux make build VERSION="${VERSION}"

FROM quay.io/hummingbird/core-runtime:2.42@sha256:c77b76cff060dde12a816e12e70dee6fabb6e93301d867a969e33ca411cef5ef

WORKDIR /app

COPY --from=build /app/forgejo-mcp .

ENTRYPOINT ["/app/forgejo-mcp"]
