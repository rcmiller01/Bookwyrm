FROM golang:1.25-alpine AS builder

WORKDIR /build

COPY go.mod go.sum ./
RUN go mod download

COPY . .

ARG VERSION=dev
ARG COMMIT=unknown
ARG BUILD_DATE=unknown

RUN CGO_ENABLED=0 go build \
    -ldflags "-X metadata-service/internal/version.Version=${VERSION} -X metadata-service/internal/version.Commit=${COMMIT} -X metadata-service/internal/version.BuildDate=${BUILD_DATE}" \
    -o metadata-service ./cmd/server

FROM alpine:3.20

WORKDIR /app

COPY --from=builder /build/metadata-service .
COPY configs/ configs/

EXPOSE 8080

CMD ["./metadata-service"]
