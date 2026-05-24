FROM registry.access.redhat.com/hi/go:1.25.10-builder@sha256:21068a8473d5d5808c76569492328fcd9764706468b5b2e42a14a794e6f35daa AS build

RUN dnf install -y make git && dnf clean all

WORKDIR /app

COPY go.mod go.sum ./
RUN go mod download

COPY . .

RUN CGO_ENABLED=0 GOOS=linux make build

FROM registry.access.redhat.com/hi/core-runtime:2.42@sha256:9666b5aad217d503660028879a72d4a500fbcbff89fcad5d1974e029eb5df4d3

WORKDIR /app

COPY --from=build /app/forgejo-mcp .

ENTRYPOINT ["/app/forgejo-mcp"]
