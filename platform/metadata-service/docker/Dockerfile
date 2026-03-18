FROM golang:1.25-alpine AS builder

WORKDIR /build

COPY go.mod go.sum ./
RUN go mod download

COPY . .
RUN CGO_ENABLED=0 go build -o metadata-service ./cmd/server

FROM alpine:3.20

WORKDIR /app

COPY --from=builder /build/metadata-service .
COPY configs/ configs/

EXPOSE 8080

CMD ["./metadata-service"]
