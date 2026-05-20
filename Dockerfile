# ── Stage 1: Build ──────────────────────────────────────────
FROM golang:1.21-alpine AS builder

RUN apk add --no-cache git

WORKDIR /app

COPY go.mod ./

# Копируем исходники (go.sum если есть — удалим и пересоздадим)
COPY . .

# Пересоздаём go.sum внутри контейнера чтобы избежать конфликта хешей
RUN rm -f go.sum && go mod tidy

RUN CGO_ENABLED=0 GOOS=linux go build -ldflags="-s -w" -o deploy-service .

# ── Stage 2: Runtime ────────────────────────────────────────
FROM alpine:3.19

# docker-cli + compose plugin + git + bash
RUN apk add --no-cache bash ca-certificates tzdata docker-cli docker-cli-compose git openssh-client curl

WORKDIR /app

COPY --from=builder /app/deploy-service .
COPY --from=builder /app/static ./static

RUN mkdir -p /opt/scripts

EXPOSE 8080

CMD ["./deploy-service"]
