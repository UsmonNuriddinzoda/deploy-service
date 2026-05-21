# DeployHub — deploy-service

Центральный HTTP-сервис на Go для удалённого деплоя любого количества сервисов через защищённый веб-интерфейс и REST API.

---

## Возможности

- 🚀 **Деплой** — запуск bash-скриптов на сервере через UI или API со стримингом вывода
- 📋 **Логи** — live-стрим `docker logs -f` прямо в браузере (SSE)
- 📊 **Статус** — просмотр состояния Docker-контейнера (`running`, `exited`, `not found`)
- ➕ **CRUD сервисов** — добавление/редактирование/удаление через UI или API
- 🔐 **Авторизация** — вход по логину/паролю (сессионный cookie) или Bearer-токену
- 🐘 **PostgreSQL** — все сервисы хранятся в БД, автомиграция при старте

---

## Структура проекта

```
deploy-service/
├── main.go                  # Точка входа, HTTP-сервер, роуты
├── config/config.go         # Конфигурация через env-переменные
├── db/
│   ├── db.go                # Подключение к PostgreSQL + автомиграция
│   └── service_repo.go      # CRUD для таблицы services
├── registry/registry.go     # Реестр сервисов (обёртка над repo)
├── handler/
│   ├── deploy.go            # /services, /deploy, /logs, /status
│   └── auth.go              # /ui/login, /ui/logout
├── middleware/auth.go        # SessionAuth, SessionOrTokenAuth
├── runner/script.go         # Запуск скрипта (mutex + таймаут + стриминг)
├── session/session.go       # Управление сессиями
├── logger/logger.go         # Логгер
├── static/
│   ├── index.html           # Веб-интерфейс (защищён сессией)
│   └── login.html           # Страница входа
├── Dockerfile
├── docker-compose.yml
└── scripts/                 # Примеры deploy-скриптов
```

---

## Быстрый старт через Docker

```bash
# 1. Скопируйте проект на сервер
cd /crm-asr/deploy-service

# 2. При необходимости отредактируйте переменные в docker-compose.yml

# 3. Соберите и запустите
docker compose up -d --build

# 4. Откройте браузер
http://<IP-сервера>:8080
```

---

## Переменные окружения

| Переменная       | Обязательна | По умолчанию | Описание                         |
|------------------|-------------|--------------|----------------------------------|
| `DATABASE_URL`   | ✅           | —            | DSN PostgreSQL                   |
| `SECRET_TOKEN`   | ✅           | —            | Bearer-токен для API             |
| `UI_USERNAME`    | Нет         | `admin`      | Логин для входа в UI             |
| `UI_PASSWORD`    | Нет         | `secret`     | Пароль для входа в UI            |
| `PORT`           | Нет         | `8080`       | Порт HTTP-сервера                |
| `SCRIPT_TIMEOUT` | Нет         | `600`        | Таймаут выполнения скрипта (сек) |

---

## Структура таблицы `services`

```sql
CREATE TABLE services (
    name        VARCHAR(100) PRIMARY KEY,       -- уникальное имя
    description TEXT         NOT NULL DEFAULT '',
    script      TEXT         NOT NULL,           -- путь к bash-скрипту на сервере
    container   VARCHAR(200) NOT NULL DEFAULT '', -- Docker-контейнер (для логов/статуса)
    created_at  TIMESTAMPTZ  NOT NULL DEFAULT NOW(),
    updated_at  TIMESTAMPTZ  NOT NULL DEFAULT NOW()
);
```

> Миграция выполняется автоматически при старте сервиса.

---

## Роуты

### UI

| Метод  | Путь         | Описание                              |
|--------|--------------|---------------------------------------|
| `GET`  | `/`          | Веб-интерфейс (требует авторизации)   |
| `GET`  | `/login`     | Страница входа                        |
| `POST` | `/ui/login`  | Авторизация (логин/пароль)            |
| `POST` | `/ui/logout` | Выход                                 |
| `GET`  | `/health`    | Проверка состояния (без авторизации)  |

### Сервисы (CRUD)

| Метод    | Путь                | Описание            |
|----------|---------------------|---------------------|
| `GET`    | `/services`         | Список всех сервисов|
| `POST`   | `/services`         | Создать сервис      |
| `GET`    | `/services/{name}`  | Получить сервис     |
| `PUT`    | `/services/{name}`  | Обновить сервис     |
| `DELETE` | `/services/{name}`  | Удалить сервис      |

### Деплой

| Метод  | Путь                         | Описание                                   |
|--------|------------------------------|--------------------------------------------|
| `POST` | `/deploy/{service}`          | Запустить деплой, вернуть JSON по завершении |
| `GET`  | `/deploy/{service}/stream`   | Запустить деплой со стримингом вывода (SSE) |

### Логи и статус Docker

| Метод | Путь                      | Описание                                     |
|-------|---------------------------|----------------------------------------------|
| `GET` | `/logs/{service}/stream`  | Live-стрим `docker logs -f --tail=200` (SSE) |
| `GET` | `/status/{service}`       | Статус контейнера через `docker inspect`     |

---

## Примеры API

### Создать сервис

```bash
curl -X POST http://localhost:8080/services \
  -H "Authorization: Bearer my-secret-token" \
  -H "Content-Type: application/json" \
  -d '{
    "name": "crm-backend",
    "description": "CRM Backend API",
    "script": "/crm-asr/scripts/deploy_crm.sh",
    "container": "crm_backend_app"
  }'
```

### Запустить деплой

```bash
curl -X POST http://localhost:8080/deploy/crm-backend \
  -H "Authorization: Bearer my-secret-token"
```

**Ответ (200):**
```json
{
  "service": "crm-backend",
  "success": true,
  "stdout": "=== Deploy started ===\ngit pull...\n",
  "stderr": "",
  "duration": "12.3s",
  "message": "deploy completed successfully"
}
```

### Получить статус контейнера

```bash
curl http://localhost:8080/status/crm-backend \
  -H "Authorization: Bearer my-secret-token"
```

**Ответ:**
```json
{
  "service": "crm-backend",
  "container": "crm_backend_app",
  "status": "running",
  "running": true,
  "image": "crm-backend:latest",
  "started_at": "2026-05-21T10:00:00Z",
  "finished_at": "0001-01-01T00:00:00Z",
  "exit_code": 0
}
```

### Стрим логов

```bash
curl -N http://localhost:8080/logs/crm-backend/stream \
  -H "Authorization: Bearer my-secret-token"
```

SSE события:
- `event: stdout` / `event: stderr` — строка лога
- `event: done` — поток завершён
- `event: error` — ошибка (контейнер не найден и т.д.)

---

## Веб-интерфейс

После входа на `http://<сервер>:8080` доступны:

- **🚀 Deploy** — запустить скрипт со стримингом вывода в браузере
- **📋 Логи** — live-просмотр `docker logs -f` контейнера *(кнопка видна если указан контейнер)*
- **📊 Статус** — проверить состояние контейнера *(кнопка видна если указан контейнер)*
- **✏️** — редактировать сервис (скрипт, контейнер, описание)
- **🗑️** — удалить сервис

### Добавление сервиса

1. Нажмите **＋ Добавить сервис**
2. Заполните поля:
   - **Название** — уникальный ID (латиница, цифры, `-`, `_`)
   - **Путь к скрипту** — абсолютный путь к bash-скрипту на сервере
   - **Docker контейнер** — имя контейнера для логов/статуса *(необязательно)*
   - **Описание** — произвольный текст *(необязательно)*

---

## Пример deploy-скрипта

```bash
#!/usr/bin/env bash
set -euo pipefail

REPO_DIR="/crm-asr/my-service"
COMPOSE_FILE="$REPO_DIR/docker-compose.yml"
LOG_PREFIX="[$(date '+%H:%M:%S')]"

echo "$LOG_PREFIX 🚀 Начинаем деплой..."
cd "$REPO_DIR"

echo "$LOG_PREFIX 📥 Получаем изменения из GitHub..."
git fetch --all
git reset --hard origin/$(git rev-parse --abbrev-ref HEAD)

echo "$LOG_PREFIX 🔨 Собираем Docker-образ..."
docker compose -f "$COMPOSE_FILE" build

echo "$LOG_PREFIX 🔄 Перезапускаем контейнеры..."
docker compose -f "$COMPOSE_FILE" up -d --remove-orphans

echo "$LOG_PREFIX ✅ Деплой завершён"
```

---

## Важные замечания

- **Docker socket**: для работы логов и статуса сервис должен иметь доступ к `/var/run/docker.sock`.  
  В `docker-compose.yml` уже настроен volume:
  ```yaml
  volumes:
    - /var/run/docker.sock:/var/run/docker.sock
  ```
- **Скрипты**: должны находиться на хосте (или в volume) и быть исполняемыми (`chmod +x script.sh`)
- **Поле `container`**: если не указано — кнопки «Логи» и «Статус» в UI не отображаются
