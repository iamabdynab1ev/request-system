-- +goose Up
-- +goose StatementBegin
SELECT 'up: enhancing order_history table';

-- 1. Добавляем поле metadata типа JSONB для надежных отчетов
ALTER TABLE public.order_history ADD COLUMN IF NOT EXISTS metadata JSONB;

-- 2. Добавляем индекс для быстрой загрузки истории ОДНОЙ заявки
CREATE INDEX IF NOT EXISTS idx_order_history_order_id_created_at ON public.order_history(order_id, created_at DESC);

-- 3. Добавляем GIN-индекс для быстрого поиска внутри metadata
CREATE INDEX IF NOT EXISTS idx_order_history_metadata_gin ON public.order_history USING GIN (metadata);

-- +goose StatementEnd

-- +goose Down
-- +goose StatementBegin
SELECT 'down: reverting order_history enhancements';

DROP INDEX IF EXISTS idx_order_history_metadata_gin;
DROP INDEX IF EXISTS idx_order_history_order_id_created_at;
ALTER TABLE public.order_history DROP COLUMN IF EXISTS metadata;

-- +goose StatementEnd
