FROM quay.io/hummingbird/go:1.27.4-builder@sha256:e05309707d18faef3294bda773ac33aebceacf80b7e56a41dda74a182edad160 AS build

# Version is injected at build time; the container has no usable .git to derive
# it from (see `make container`). Defaults to "dev" for plain `podman build`.
ARG VERSION=dev

RUN dnf install -y make git && dnf clean all

WORKDIR /app

COPY go.mod go.sum ./
RUN go mod download

COPY . .

RUN CGO_ENABLED=0 GOOS=linux make build VERSION="${VERSION}"

FROM quay.io/hummingbird/core-runtime:2.43@sha256:b89211eabec2378de257b754643579aabb9b87bc67ed8411dcef0b9ef8142d68

WORKDIR /app

COPY --from=build /app/forgejo-mcp .

ENTRYPOINT ["/app/forgejo-mcp"]
