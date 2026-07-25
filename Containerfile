FROM quay.io/hummingbird/go:1.26.5-builder@sha256:450f2624c86211e42c6b6619992556c2dc95101de4e441737a4254d9ed1cafd1 AS build

# Version is injected at build time; the container has no usable .git to derive
# it from (see `make container`). Defaults to "dev" for plain `podman build`.
ARG VERSION=dev

RUN dnf install -y make git && dnf clean all

WORKDIR /app

COPY go.mod go.sum ./
RUN go mod download

COPY . .

RUN CGO_ENABLED=0 GOOS=linux make build VERSION="${VERSION}"

FROM quay.io/hummingbird/core-runtime:2.43@sha256:04d24b99a65513db0d32bd7144d5b36a8e68dcfb0f267f74705d14ad3e01c10a

WORKDIR /app

COPY --from=build /app/forgejo-mcp .

ENTRYPOINT ["/app/forgejo-mcp"]
