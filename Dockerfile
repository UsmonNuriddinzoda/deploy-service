# ── Stage 1: Build ──────────────────────────────────────────
FROM golang:1.21-alpine AS builder

RUN apk add --no-cache git

WORKDIR /app

# Копируем только go.mod — go.sum будет сгенерирован автоматически
COPY go.mod ./
RUN go mod download && go mod tidy 2>/dev/null || true

# Исходники
COPY . .

# go.sum не копируется (.dockerignore), go mod tidy создаёт правильный
RUN go mod tidy && CGO_ENABLED=0 GOOS=linux go build -ldflags="-s -w" -o deploy-service .

# ── Stage 2: Runtime ────────────────────────────────────────
FROM alpine:3.19

RUN apk add --no-cache bash ca-certificates tzdata

WORKDIR /app

COPY --from=builder /app/deploy-service .
COPY --from=builder /app/static ./static

RUN mkdir -p /opt/scripts

EXPOSE 8080

CMD ["./deploy-service"]
