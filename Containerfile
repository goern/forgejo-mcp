FROM quay.io/hummingbird/go:1.26.5-builder@sha256:152f522ba7809fc547bcd40935693db0a4cc47fbccf913959cca122af59c8fc7 AS build

# Version is injected at build time; the container has no usable .git to derive
# it from (see `make container`). Defaults to "dev" for plain `podman build`.
ARG VERSION=dev

RUN dnf install -y make git && dnf clean all

WORKDIR /app

COPY go.mod go.sum ./
RUN go mod download

COPY . .

RUN CGO_ENABLED=0 GOOS=linux make build VERSION="${VERSION}"

FROM quay.io/hummingbird/core-runtime:2.43@sha256:32b71590eef9295afa862261d66f90b9b4c537df59e35684e33a9e6bbae03c5a

WORKDIR /app

COPY --from=build /app/forgejo-mcp .

ENTRYPOINT ["/app/forgejo-mcp"]
