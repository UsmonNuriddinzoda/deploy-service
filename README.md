# deploy-service

Центральный HTTP-сервис на Go для удалённого деплоя **любого количества сервисов** через защищённые роуты.

## Структура проекта

```
deploy-service/
├── main.go               # Точка входа, HTTP сервер, graceful shutdown
├── config/config.go      # Конфигурация через env-переменные
├── registry/registry.go  # Реестр сервисов из services.json
├── handler/deploy.go     # Обработчики роутов
├── middleware/auth.go    # Проверка Bearer-токена
├── runner/script.go      # Запуск скрипта (per-service mutex + таймаут)
├── logger/logger.go      # Логгер
├── services.json         # Список сервисов (редактируйте!)
├── deploy.sh             # Пример скрипта деплоя
├── .env.example          # Пример переменных окружения
└── go.mod
```

---

## Добавление нового сервиса

Просто добавьте запись в `services.json`:

```json
[
  {
    "name": "backend",
    "description": "Backend API",
    "script": "/opt/scripts/deploy-backend.sh"
  },
  {
    "name": "frontend",
    "description": "Frontend React",
    "script": "/opt/scripts/deploy-frontend.sh"
  },
  {
    "name": "bot",
    "description": "Telegram Bot",
    "script": "/opt/scripts/deploy-bot.sh"
  }
]
```

Поля:
| Поле          | Обязательно | Описание                    |
|---------------|-------------|-----------------------------|
| `name`        | ✅          | Уникальное имя сервиса      |
| `script`      | ✅          | Путь к bash/ps1 скрипту     |
| `description` | Нет         | Описание (для /services)    |

---

## Быстрый старт

```bash
# 1. Сборка
go build -o deploy-service .

# 2. Переменные окружения
export SECRET_TOKEN="my-secret-token"
export PORT=8080
export SERVICES_FILE="./services.json"

# 3. Запуск
./deploy-service
```

---

## Роуты

### `POST /deploy/{service}`
Запускает скрипт деплоя для указанного сервиса.

```bash
# Деплой backend
curl -X POST http://localhost:8080/deploy/backend \
  -H "Authorization: Bearer my-secret-token"

# Деплой frontend
curl -X POST http://localhost:8080/deploy/frontend \
  -H "Authorization: Bearer my-secret-token"

# Деплой bot
curl -X POST http://localhost:8080/deploy/bot \
  -H "Authorization: Bearer my-secret-token"
```

**Успешный ответ (200):**
```json
{
  "service": "backend",
  "success": true,
  "stdout": "=== Deploy started ===\ngit pull...\n",
  "stderr": "",
  "duration": "4.2s",
  "message": "deploy completed successfully"
}
```

**Сервис не найден (404):**
```json
{ "error": "service 'unknown' not found in registry" }
```

**Деплой уже запущен (500):**
```json
{
  "service": "backend",
  "success": false,
  "stderr": "deploy of 'backend' already in progress, please try again later",
  ...
}
```

---

### `GET /services`
Список всех зарегистрированных сервисов.

```bash
curl http://localhost:8080/services \
  -H "Authorization: Bearer my-secret-token"
```

```json
[
  { "name": "backend",  "description": "Backend API",     "script": "/opt/scripts/deploy-backend.sh" },
  { "name": "frontend", "description": "Frontend React",  "script": "/opt/scripts/deploy-frontend.sh" },
  { "name": "bot",      "description": "Telegram Bot",    "script": "/opt/scripts/deploy-bot.sh" }
]
```

---

### `GET /health`
Проверка состояния (без авторизации).

```bash
curl http://localhost:8080/health
# {"status":"ok"}
```

---

## Переменные окружения

| Переменная       | Обязательна | По умолчанию        | Описание                          |
|------------------|-------------|---------------------|-----------------------------------|
| `SECRET_TOKEN`   | ✅ Да        | —                   | Токен авторизации                 |
| `PORT`           | Нет         | `8080`              | Порт HTTP-сервера                 |
| `SERVICES_FILE`  | Нет         | `./services.json`   | Путь к файлу реестра сервисов     |
| `SCRIPT_TIMEOUT` | Нет         | `600`               | Таймаут скрипта в секундах        |

---

## Пример скрипта деплоя (`/opt/scripts/deploy-backend.sh`)

```bash
#!/bin/bash
set -e

echo "=== [backend] Deploy started at $(date) ==="
cd /var/www/backend
git pull origin main
go build -o ./bin/app .
sudo systemctl restart backend
echo "=== [backend] Deploy finished at $(date) ==="
```

---

## Запуск как systemd-сервис

```ini
[Unit]
Description=Deploy Service
After=network.target

[Service]
Type=simple
User=www-data
WorkingDirectory=/opt/deploy-service
ExecStart=/opt/deploy-service/deploy-service
Environment=SECRET_TOKEN=your-secret-token
Environment=PORT=8080
Environment=SERVICES_FILE=/opt/deploy-service/services.json
Restart=always

[Install]
WantedBy=multi-user.target
```

```bash
sudo systemctl daemon-reload
sudo systemctl enable deploy-service
sudo systemctl start deploy-service
```
