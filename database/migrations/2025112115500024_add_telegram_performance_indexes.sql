-- Миграция: добавление индексов для оптимизации Telegram запросов
-- Файл: migrations/YYYYMMDDHHMMSS_add_telegram_performance_indexes.sql

-- +goose Up
-- Индексы для быстрого поиска заявок через Telegram
CREATE INDEX IF NOT EXISTS idx_orders_executor_id 
ON orders(executor_id) 
WHERE executor_id IS NOT NULL AND deleted_at IS NULL;

CREATE INDEX IF NOT EXISTS idx_orders_creator_id 
ON orders(user_id) 
WHERE deleted_at IS NULL;

CREATE INDEX IF NOT EXISTS idx_orders_status_created 
ON orders(status_id, created_at DESC) 
WHERE deleted_at IS NULL;

-- Составной индекс для запросов "Мои заявки" (creator OR executor)
CREATE INDEX IF NOT EXISTS idx_orders_participant_lookup 
ON orders(user_id, executor_id, created_at DESC, status_id) 
WHERE deleted_at IS NULL;

-- Индекс для поиска просроченных заявок
CREATE INDEX IF NOT EXISTS idx_orders_duration_status 
ON orders(duration, status_id) 
WHERE deleted_at IS NULL AND duration IS NOT NULL;

-- Индекс для истории заявок (проверка участия)
CREATE INDEX IF NOT EXISTS idx_order_history_user_order 
ON order_history(user_id, order_id);

-- +goose Down
DROP INDEX IF EXISTS idx_orders_executor_id;
DROP INDEX IF EXISTS idx_orders_creator_id;
DROP INDEX IF EXISTS idx_orders_status_created;
DROP INDEX IF EXISTS idx_orders_participant_lookup;
DROP INDEX IF EXISTS idx_orders_duration_status;
DROP INDEX IF EXISTS idx_order_history_user_order;
