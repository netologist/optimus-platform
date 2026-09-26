# Minimal lightweight Dockerfile for Optimus Platform API
# TODO: In production, split into separate images for cmd/api, cmd/worker, and cmd/outbox-relay
FROM golang:1.24-alpine AS builder
WORKDIR /app
ENV CGO_ENABLED=0 GOTOOLCHAIN=auto
COPY go.mod go.sum ./
RUN go mod download
COPY . .
RUN go build -ldflags="-s -w" -o /app/platform-api ./cmd/api/main.go

FROM alpine:3.20
WORKDIR /app
COPY --from=builder /app/platform-api /app/platform-api
EXPOSE 8080
ENTRYPOINT ["/app/platform-api"]
