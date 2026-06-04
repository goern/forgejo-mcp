FROM quay.io/hummingbird/go:1.26.4-builder@sha256:433377461114118a3466e4acd8807c3a53b14a5d855909c9eb927273ef9d6c9a AS build

# Version is injected at build time; the container has no usable .git to derive
# it from (see `make container`). Defaults to "dev" for plain `podman build`.
ARG VERSION=dev

RUN dnf install -y make git && dnf clean all

WORKDIR /app

COPY go.mod go.sum ./
RUN go mod download

COPY . .

RUN CGO_ENABLED=0 GOOS=linux make build VERSION="${VERSION}"

FROM quay.io/hummingbird/core-runtime:2.42@sha256:c85f5e01b7f638cb30e75a8a79d06b0cbeb44209945f62572166448bb56b53e9

WORKDIR /app

COPY --from=build /app/forgejo-mcp .

ENTRYPOINT ["/app/forgejo-mcp"]
