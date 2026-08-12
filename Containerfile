FROM quay.io/hummingbird/go:1.26.5-builder@sha256:5f331df1581236847948d1d9e9ad8d9c9dd36bb3ffd6cb88f9be46593469af31 AS build

# Version is injected at build time; the container has no usable .git to derive
# it from (see `make container`). Defaults to "dev" for plain `podman build`.
ARG VERSION=dev

RUN dnf install -y make git && dnf clean all

WORKDIR /app

COPY go.mod go.sum ./
RUN go mod download

COPY . .

RUN CGO_ENABLED=0 GOOS=linux make build VERSION="${VERSION}"

FROM quay.io/hummingbird/core-runtime:2.43@sha256:4ba896089e33ea92d6ca4b03194fc3e88010077fc840d859a7893e4d5b9fd6f6

WORKDIR /app

COPY --from=build /app/forgejo-mcp .

ENTRYPOINT ["/app/forgejo-mcp"]
