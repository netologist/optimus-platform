# Minimal lightweight Dockerfile for Optimus Enterprise Mocks
# TODO: In production, split into independently scaled deployments per system (EAM, PLM, ERP, FSM, OI)
FROM golang:1.24-alpine AS builder
WORKDIR /app
COPY go.mod go.sum* ./
RUN go mod download || true
COPY . .
RUN CGO_ENABLED=0 go build -ldflags="-s -w" -o /app/integration-mocks ./cmd/main.go

FROM alpine:3.20
WORKDIR /app
COPY --from=builder /app/integration-mocks /app/integration-mocks
EXPOSE 8080
ENTRYPOINT ["/app/integration-mocks"]
CMD ["--system=all", "--port=8080"]
