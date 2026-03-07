FROM golang:1.25-alpine AS builder

WORKDIR /build

COPY go.mod go.sum ./
RUN go mod download

COPY . .

ARG VERSION=dev
ARG COMMIT=unknown
ARG BUILD_DATE=unknown

RUN CGO_ENABLED=0 go build \
    -ldflags "-X indexer-service/internal/version.Version=${VERSION} -X indexer-service/internal/version.Commit=${COMMIT} -X indexer-service/internal/version.BuildDate=${BUILD_DATE}" \
    -o indexer-service ./cmd/server

FROM alpine:3.20

WORKDIR /app

COPY --from=builder /build/indexer-service .

EXPOSE 8091

CMD ["./indexer-service"]
