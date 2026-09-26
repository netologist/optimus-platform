# Minimal lightweight Dockerfile for Optimus Kubernetes Operator
# TODO: In production, configure admission webhooks and leader election flags
FROM golang:1.24-alpine AS builder
WORKDIR /app
ENV CGO_ENABLED=0 GOTOOLCHAIN=auto
COPY go.mod go.sum* ./
RUN go mod download || true
COPY . .
RUN go build -ldflags="-s -w" -o /app/operator ./cmd/main.go

FROM alpine:3.20
WORKDIR /app
COPY --from=builder /app/operator /app/operator
USER 65532:65532
ENTRYPOINT ["/app/operator"]
