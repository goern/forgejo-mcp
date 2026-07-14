FROM quay.io/hummingbird/go:1.26.5-builder@sha256:0ab57a83abad25f7207fc60d9bbb75a65835684d476613705284b3fc59d623fe AS build

# Version is injected at build time; the container has no usable .git to derive
# it from (see `make container`). Defaults to "dev" for plain `podman build`.
ARG VERSION=dev

RUN dnf install -y make git && dnf clean all

WORKDIR /app

COPY go.mod go.sum ./
RUN go mod download

COPY . .

RUN CGO_ENABLED=0 GOOS=linux make build VERSION="${VERSION}"

FROM quay.io/hummingbird/core-runtime:2.43@sha256:3878ecc9403612958486026c6ab55c31a6f677ab07290c2673a39b8933908d50

WORKDIR /app

COPY --from=build /app/forgejo-mcp .

ENTRYPOINT ["/app/forgejo-mcp"]
