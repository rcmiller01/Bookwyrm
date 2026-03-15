FROM golang:1.25-alpine AS builder

WORKDIR /build

COPY go.mod go.sum ./
RUN go mod download

COPY . .
RUN CGO_ENABLED=0 GOOS=linux go build -o indexer-service ./cmd/server

FROM alpine:3.20

WORKDIR /app

COPY --from=builder /build/indexer-service ./indexer-service

EXPOSE 8091

CMD ["./indexer-service"]
