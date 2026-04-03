-- +goose Up
-- +goose StatementBegin
SELECT 'up: adding kpi columns to orders table for fast dashboards';

-- 1. Время фактического завершения заявки
ALTER TABLE public.orders ADD COLUMN IF NOT EXISTS completed_at TIMESTAMPTZ;

-- 2. Время решения в секундах (для быстрого AVG() на дашборде)
ALTER TABLE public.orders ADD COLUMN IF NOT EXISTS resolution_time_seconds INTEGER;

-- 3. Время до первого ответа/действия в секундах (для KPI "Среднее время ответа")
ALTER TABLE public.orders ADD COLUMN IF NOT EXISTS first_response_time_seconds INTEGER;

-- 4. (Опционально, но полезно для KPI "Решение с первого раза")
ALTER TABLE public.orders ADD COLUMN IF NOT EXISTS is_first_contact_resolution BOOLEAN;

-- +goose StatementEnd

-- +goose Down
-- +goose StatementBegin
SELECT 'down: removing kpi columns from orders table';

ALTER TABLE public.orders DROP COLUMN IF EXISTS is_first_contact_resolution;
ALTER TABLE public.orders DROP COLUMN IF EXISTS first_response_time_seconds;
ALTER TABLE public.orders DROP COLUMN IF EXISTS resolution_time_seconds;
ALTER TABLE public.orders DROP COLUMN IF EXISTS completed_at;

-- +goose StatementEnd
