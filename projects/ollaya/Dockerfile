FROM golang:1.24-alpine AS builder
WORKDIR /app
COPY main.go .
RUN CGO_ENABLED=0 go build -ldflags="-s -w" -o ollaya-server main.go

FROM alpine:3.20
WORKDIR /app
COPY --from=builder /app/ollaya-server .
EXPOSE 11435
ENTRYPOINT ["/app/ollaya-server"]
