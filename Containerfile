FROM golang:1.25-alpine@sha256:6032270f7aa8f5831fb69a1024b1b21898b986e8073c71781437cfc0c5f49051 AS build

RUN apk --no-cache add make

WORKDIR /app

COPY go.mod go.sum ./
RUN go mod download

COPY . .

RUN CGO_ENABLED=0 GOOS=linux make

FROM alpine:edge@sha256:115729ec5cb049ba6359c3ab005ac742012d92bbaa5b8bc1a878f1e8f62c0cb8

WORKDIR /app

COPY --from=build /app/forgejo-mcp .

RUN apk --no-cache add ca-certificates tzdata

ENTRYPOINT ["/app/forgejo-mcp"]
