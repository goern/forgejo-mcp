FROM golang:1.26-alpine@sha256:e58f92c51f4cc98f2ef4780ba8708fe914f20e6175720a19c1c85ff029a6b9fc AS build

RUN apk --no-cache add make

WORKDIR /app

COPY go.mod go.sum ./
RUN go mod download

COPY . .

RUN CGO_ENABLED=0 GOOS=linux make

FROM alpine:edge@sha256:9a341ff2287c54b86425cbee0141114d811ae69d88a36019087be6d896cef241

WORKDIR /app

COPY --from=build /app/forgejo-mcp .

RUN apk --no-cache add ca-certificates tzdata

ENTRYPOINT ["/app/forgejo-mcp"]
