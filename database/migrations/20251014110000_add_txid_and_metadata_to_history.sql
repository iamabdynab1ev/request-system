-- +goose Up
-- +goose StatementBegin
SELECT 'up: adding tx_id column to order_history';

-- Добавляем колонку tx_id для группировки связанных событий истории.
ALTER TABLE public.order_history
ADD COLUMN IF NOT EXISTS tx_id UUID;

-- Добавляем индекс для этой колонки для быстрого поиска.
CREATE INDEX IF NOT EXISTS idx_order_history_tx_id ON public.order_history(tx_id);

-- +goose StatementEnd

-- +goose Down
-- +goose StatementBegin
SELECT 'down: removing tx_id column from order_history';

-- Откатываем в обратном порядке.
DROP INDEX IF EXISTS idx_order_history_tx_id;
ALTER TABLE public.order_history DROP COLUMN IF EXISTS tx_id;

-- +goose StatementEnd
