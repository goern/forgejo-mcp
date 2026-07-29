FROM quay.io/hummingbird/go:1.26.5-builder@sha256:021bbd48691134c6695a8681fb2b7a397c55b7c7ef7c85e76d302e7e90b37fea AS build

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
