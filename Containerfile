FROM quay.io/hummingbird/go:1.26.5-builder@sha256:aef155087940a9221c59ca48ad8af371a4c1a46b05f0283737149cb00ef484b3 AS build

# Version is injected at build time; the container has no usable .git to derive
# it from (see `make container`). Defaults to "dev" for plain `podman build`.
ARG VERSION=dev

RUN dnf install -y make git && dnf clean all

WORKDIR /app

COPY go.mod go.sum ./
RUN go mod download

COPY . .

RUN CGO_ENABLED=0 GOOS=linux make build VERSION="${VERSION}"

FROM quay.io/hummingbird/core-runtime:2.43@sha256:ba6d97c401f569b74ff26ac58acd35428f2a49d8e48db4976eaa63a6879a445f

WORKDIR /app

COPY --from=build /app/forgejo-mcp .

ENTRYPOINT ["/app/forgejo-mcp"]
