-- +goose Up
-- +goose StatementBegin
SELECT 'up: adding telegram_chat_id to users table';

-- Добавляем колонку для хранения ID чата в Telegram
ALTER TABLE public.users
ADD COLUMN telegram_chat_id BIGINT NULL;

-- Добавляем UNIQUE ограничение, чтобы один Telegram-аккаунт
-- не мог быть привязан к нескольким пользователям системы.
ALTER TABLE public.users
ADD CONSTRAINT users_telegram_chat_id_unique UNIQUE (telegram_chat_id);

-- +goose StatementEnd

-- +goose Down
-- +goose StatementBegin
SELECT 'down: removing telegram_chat_id from users table';

-- Удаляем ограничение и колонку
ALTER TABLE public.users
DROP CONSTRAINT IF EXISTS users_telegram_chat_id_unique;

ALTER TABLE public.users
DROP COLUMN IF EXISTS telegram_chat_id;

-- +goose StatementEnd

