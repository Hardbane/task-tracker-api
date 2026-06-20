# syntax=docker/dockerfile:1

FROM golang:1.23-alpine AS builder
WORKDIR /src

RUN apk add --no-cache git ca-certificates

COPY go.mod ./
COPY go.sum* ./
RUN go mod download

COPY . .

RUN go mod tidy

RUN CGO_ENABLED=0 GOOS=linux GOARCH=amd64 go build -trimpath -ldflags="-s -w" -o /bin/task-api ./cmd/api

FROM alpine:3.20
WORKDIR /app

RUN apk add --no-cache ca-certificates tzdata && adduser -D -H -u 10001 appuser

COPY --from=builder /bin/task-api /app/task-api
COPY config.yaml /app/config.yaml
COPY migrations /app/migrations

USER appuser
EXPOSE 8080

CMD ["/app/task-api", "--config", "/app/config.yaml"]