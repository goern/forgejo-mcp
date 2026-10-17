FROM quay.io/hummingbird/go:1.27.2-builder@sha256:780d920a7f2e768e26b6d3fe95673908ee552e5dbcc45d1a17125c1b06dcf718 AS build

# Version is injected at build time; the container has no usable .git to derive
# it from (see `make container`). Defaults to "dev" for plain `podman build`.
ARG VERSION=dev

RUN dnf install -y make git && dnf clean all

WORKDIR /app

COPY go.mod go.sum ./
RUN go mod download

COPY . .

RUN CGO_ENABLED=0 GOOS=linux make build VERSION="${VERSION}"

FROM quay.io/hummingbird/core-runtime:2.43@sha256:cefdad691990bab9d5371070824c1bb52b9a0a9733eaf543eded2a6964715881

WORKDIR /app

COPY --from=build /app/forgejo-mcp .

ENTRYPOINT ["/app/forgejo-mcp"]
