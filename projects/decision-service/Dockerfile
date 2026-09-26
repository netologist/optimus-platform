# Minimal lightweight Dockerfile for Optimus Decision Service
# TODO: In production, configure mTLS and hardware acceleration for hosted Ollaya/TypeSafe
FROM golang:1.24-alpine AS builder
WORKDIR /app
ENV CGO_ENABLED=0 GOTOOLCHAIN=auto
COPY go.mod go.sum* ./
RUN go mod download || true
COPY . .
RUN go build -ldflags="-s -w" -o /app/decision-service ./cmd/main.go

FROM alpine:3.20
WORKDIR /app
COPY --from=builder /app/decision-service /app/decision-service
EXPOSE 8082
ENTRYPOINT ["/app/decision-service"]
