FROM quay.io/hummingbird/go:1.26.5-builder@sha256:990373bb0947747bc6cd391015f6175c2730bf5ae620d65a8331d93c41757e91 AS build

# Version is injected at build time; the container has no usable .git to derive
# it from (see `make container`). Defaults to "dev" for plain `podman build`.
ARG VERSION=dev

RUN dnf install -y make git && dnf clean all

WORKDIR /app

COPY go.mod go.sum ./
RUN go mod download

COPY . .

RUN CGO_ENABLED=0 GOOS=linux make build VERSION="${VERSION}"

FROM quay.io/hummingbird/core-runtime:2.43@sha256:b1cc97b3fee6c84407bea32d4d638a898c160f6682179ef03e56d90af44cfc1f

WORKDIR /app

COPY --from=build /app/forgejo-mcp .

ENTRYPOINT ["/app/forgejo-mcp"]
