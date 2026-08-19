FROM quay.io/hummingbird/go:1.26.6-builder@sha256:b157afcc05a77d0b972a21678b6672dabdace3caf2f06394a41b70419c6ced49 AS build

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
