#!/bin/sh

# Эта строчка не дает скрипту продолжаться, если какая-либо команда провалится
set -e

# Формируем строку подключения из переменных окружения
export DB_DSN="$DB_USER:$DB_PASSWORD@$DB_HOST:$DB_PORT/$DB_NAME?sslmode=disable"

# Не печатаем DSN целиком, чтобы не светить пароль в логах
echo "Running migrations..."

# Запускаем goose с готовой строкой
goose -dir "database/migrations" postgres "postgres://$DB_DSN" up

echo "Migrations applied successfully!"
