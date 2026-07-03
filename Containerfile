FROM quay.io/hummingbird/go:1.26.4-builder@sha256:d1c6067b6538ae54fda70c6cf4d93ceb411cbc1ba38a27a50276b237253eaef2 AS build

# Version is injected at build time; the container has no usable .git to derive
# it from (see `make container`). Defaults to "dev" for plain `podman build`.
ARG VERSION=dev

RUN dnf install -y make git && dnf clean all

WORKDIR /app

COPY go.mod go.sum ./
RUN go mod download

COPY . .

RUN CGO_ENABLED=0 GOOS=linux make build VERSION="${VERSION}"

FROM quay.io/hummingbird/core-runtime:2.42@sha256:b93bfca801245219c332093e1c52a639414154533cecb1522630aeef48710960

WORKDIR /app

COPY --from=build /app/forgejo-mcp .

ENTRYPOINT ["/app/forgejo-mcp"]
