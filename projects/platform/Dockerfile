# Dockerfile for Optimus Platform API, Temporal Worker & Outbox Relay
# Base image must satisfy the `go` directive in go.mod, otherwise GOTOOLCHAIN=auto
# downloads a toolchain on every build.
FROM golang:1.26-alpine AS builder
WORKDIR /app
ENV CGO_ENABLED=0 GOTOOLCHAIN=auto
COPY go.mod go.sum ./
RUN go mod download
COPY . .
RUN go build -ldflags="-s -w" -o /app/platform-api ./cmd/api/main.go && \
    go build -ldflags="-s -w" -o /app/platform-worker ./cmd/worker/main.go && \
    go build -ldflags="-s -w" -o /app/outbox-relay ./cmd/outbox-relay/main.go

FROM alpine:3.20
WORKDIR /app
COPY --from=builder /app/platform-api /app/platform-api
COPY --from=builder /app/platform-worker /app/platform-worker
COPY --from=builder /app/outbox-relay /app/outbox-relay
EXPOSE 8080
ENTRYPOINT ["/app/platform-api"]
