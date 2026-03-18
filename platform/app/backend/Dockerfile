# Stage 1: Build frontend
FROM node:22-alpine AS frontend

WORKDIR /build/web
COPY web/package.json web/package-lock.json ./
RUN npm ci
COPY web/ .
RUN npm run build

# Stage 2: Build Go binary
FROM golang:1.25-alpine AS builder

WORKDIR /build

COPY go.mod go.sum ./
RUN go mod download

COPY . .

ARG VERSION=dev
ARG COMMIT=unknown
ARG BUILD_DATE=unknown

RUN CGO_ENABLED=0 go build \
    -ldflags "-X app-backend/internal/version.Version=${VERSION} -X app-backend/internal/version.Commit=${COMMIT} -X app-backend/internal/version.BuildDate=${BUILD_DATE}" \
    -o app-backend ./cmd/server

# Stage 3: Runtime
FROM alpine:3.20

WORKDIR /app

COPY --from=builder /build/app-backend .
COPY --from=frontend /build/web/dist ./web/dist

EXPOSE 8090

CMD ["./app-backend"]
