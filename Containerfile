FROM registry.access.redhat.com/hi/go:1.26.3-builder@sha256:6bc8aae21eca50925513b4905c8740da8cb6818e3ff0ebc56d09f78f8812b6da AS build

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
