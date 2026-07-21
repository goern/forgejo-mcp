FROM quay.io/hummingbird/go:1.26.5-builder@sha256:ed2ffa394ca0313b8c59b87efb9b08ded60de40b8f94d75635092349713fa1ca AS build

# Version is injected at build time; the container has no usable .git to derive
# it from (see `make container`). Defaults to "dev" for plain `podman build`.
ARG VERSION=dev

RUN dnf install -y make git && dnf clean all

WORKDIR /app

COPY go.mod go.sum ./
RUN go mod download

COPY . .

RUN CGO_ENABLED=0 GOOS=linux make build VERSION="${VERSION}"

FROM quay.io/hummingbird/core-runtime:2.43@sha256:9eee418dffb012493df240703a4532a35df36905083b1db0e787fdfd9b327f60

WORKDIR /app

COPY --from=build /app/forgejo-mcp .

ENTRYPOINT ["/app/forgejo-mcp"]
