FROM quay.io/hummingbird/go:1.26.5-builder@sha256:050af6311e382c75b2fe8a00f6e5250d6e48925c503462a9205e282b8de50454 AS build

# Version is injected at build time; the container has no usable .git to derive
# it from (see `make container`). Defaults to "dev" for plain `podman build`.
ARG VERSION=dev

RUN dnf install -y make git && dnf clean all

WORKDIR /app

COPY go.mod go.sum ./
RUN go mod download

COPY . .

RUN CGO_ENABLED=0 GOOS=linux make build VERSION="${VERSION}"

FROM quay.io/hummingbird/core-runtime:2.42@sha256:fa6f3fdb7a2e42eb36d722930247758b88d1289ba84b5f51becbfd44656c9357

WORKDIR /app

COPY --from=build /app/forgejo-mcp .

ENTRYPOINT ["/app/forgejo-mcp"]
