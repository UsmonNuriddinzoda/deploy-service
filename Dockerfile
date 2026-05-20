# ── Stage 1: Build ──────────────────────────────────────────
FROM golang:1.21-alpine AS builder

RUN apk add --no-cache git

WORKDIR /app

# Копируем go.mod и go.sum вместе
COPY go.mod go.sum ./
RUN go mod download

# Исходники
COPY . .

RUN CGO_ENABLED=0 GOOS=linux go build -ldflags="-s -w" -o deploy-service .

# ── Stage 2: Runtime ────────────────────────────────────────
FROM alpine:3.19

RUN apk add --no-cache bash ca-certificates tzdata docker-cli docker-compose git openssh-client curl

WORKDIR /app

COPY --from=builder /app/deploy-service .
COPY --from=builder /app/static ./static

RUN mkdir -p /opt/scripts

EXPOSE 8080

CMD ["./deploy-service"]
