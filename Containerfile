FROM quay.io/hummingbird/go:1.26.4-builder@sha256:5be929c02356c97c4939e79c6da819aa2369697a13754790d5bee88f5885c432 AS build

# Version is injected at build time; the container has no usable .git to derive
# it from (see `make container`). Defaults to "dev" for plain `podman build`.
ARG VERSION=dev

RUN dnf install -y make git && dnf clean all

WORKDIR /app

COPY go.mod go.sum ./
RUN go mod download

COPY . .

RUN CGO_ENABLED=0 GOOS=linux make build VERSION="${VERSION}"

FROM quay.io/hummingbird/core-runtime:2.42@sha256:3fadedf666f137f7d36a673fcf215307bf19bc56c12eb6a323674606eac3c5bf

WORKDIR /app

COPY --from=build /app/forgejo-mcp .

ENTRYPOINT ["/app/forgejo-mcp"]
