FROM quay.io/hummingbird/go:1.27.0-builder@sha256:fc737660762a665df555c8a263bd6a1d1cefde16c80e80c321ce1ecd5645b1be AS build

# Version is injected at build time; the container has no usable .git to derive
# it from (see `make container`). Defaults to "dev" for plain `podman build`.
ARG VERSION=dev

RUN dnf install -y make git && dnf clean all

WORKDIR /app

COPY go.mod go.sum ./
RUN go mod download

COPY . .

RUN CGO_ENABLED=0 GOOS=linux make build VERSION="${VERSION}"

FROM quay.io/hummingbird/core-runtime:2.43@sha256:114d1b0ba3e2a1fc4ef42f8c1a388c6a7a2f8dca42a38a433464d0edb15bd56b

WORKDIR /app

COPY --from=build /app/forgejo-mcp .

ENTRYPOINT ["/app/forgejo-mcp"]
