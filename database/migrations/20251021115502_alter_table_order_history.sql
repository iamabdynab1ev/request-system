-- +goose Up
-- +goose StatementBegin
SELECT 'up: adding role FIO columns to order_history';

-- Добавляем колонки для хранения FIO ролей (creator, delegator, executor) для удобства отчётов и дашборда.
-- Эти поля — denormalized FIO на момент события, nullable для старых записей.
ALTER TABLE public.order_history
ADD COLUMN IF NOT EXISTS creator_fio TEXT,
ADD COLUMN IF NOT EXISTS delegator_fio TEXT,
ADD COLUMN IF NOT EXISTS executor_fio TEXT;

-- Добавляем индексы для быстрого поиска/группировки по ролям (для "Мои заявки", отчётов).
CREATE INDEX IF NOT EXISTS idx_order_history_creator_fio ON public.order_history(creator_fio);
CREATE INDEX IF NOT EXISTS idx_order_history_delegator_fio ON public.order_history(delegator_fio);
CREATE INDEX IF NOT EXISTS idx_order_history_executor_fio ON public.order_history(executor_fio);

-- +goose StatementEnd

-- +goose Down
-- +goose StatementBegin
SELECT 'down: removing role FIO columns from order_history';

-- Откатываем в обратном порядке: удаляем индексы, затем колонки.
DROP INDEX IF EXISTS idx_order_history_creator_fio;
DROP INDEX IF EXISTS idx_order_history_delegator_fio;
DROP INDEX IF EXISTS idx_order_history_executor_fio;

ALTER TABLE public.order_history 
DROP COLUMN IF EXISTS creator_fio,
DROP COLUMN IF EXISTS delegator_fio,
DROP COLUMN IF EXISTS executor_fio;

-- +goose StatementEnd
