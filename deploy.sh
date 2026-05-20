#!/bin/bash
set -e

echo "=== Deploy started at $(date) ==="

# Переходим в папку проекта (укажи свой путь)
# cd /var/www/myapp

# Получаем последние изменения
echo "--- git pull ---"
git pull origin main

# Устанавливаем зависимости (пример для Node.js)
# echo "--- npm install ---"
# npm install --production

# Перезапускаем приложение (пример для systemd)
# echo "--- restarting service ---"
# sudo systemctl restart myapp

# Или Docker:
# echo "--- docker compose up ---"
# docker compose pull && docker compose up -d

echo "=== Deploy finished at $(date) ==="

