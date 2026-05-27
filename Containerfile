FROM registry.access.redhat.com/hi/go:1.26.3-builder@sha256:6f8cd6729235b19035d569864c3eba04ff0d10a9e4229c65c017ac963bbb3a97 AS build

RUN dnf install -y make git && dnf clean all

WORKDIR /app

COPY go.mod go.sum ./
RUN go mod download

COPY . .

RUN CGO_ENABLED=0 GOOS=linux make build

FROM registry.access.redhat.com/hi/core-runtime:2.42@sha256:c85f5e01b7f638cb30e75a8a79d06b0cbeb44209945f62572166448bb56b53e9

WORKDIR /app

COPY --from=build /app/forgejo-mcp .

ENTRYPOINT ["/app/forgejo-mcp"]
