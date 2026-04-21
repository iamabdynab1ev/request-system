-- +goose Up
-- +goose StatementBegin
SELECT 'up: optimizing dashboard runtime indexes';

-- Дашборд часто фильтрует завершённые заявки по completed_at и status_id.
CREATE INDEX IF NOT EXISTS idx_orders_completed_at_desc
    ON public.orders (completed_at DESC)
    WHERE deleted_at IS NULL AND completed_at IS NOT NULL;

CREATE INDEX IF NOT EXISTS idx_orders_status_completed_at
    ON public.orders (status_id, completed_at DESC)
    WHERE deleted_at IS NULL AND completed_at IS NOT NULL;

-- Виджет last_activity сортирует историю по created_at DESC и затем джойнится к orders.
CREATE INDEX IF NOT EXISTS idx_order_history_created_at_order_id
    ON public.order_history (created_at DESC, order_id);
-- +goose StatementEnd

-- +goose Down
-- +goose StatementBegin
SELECT 'down: removing dashboard runtime indexes';

DROP INDEX IF EXISTS idx_order_history_created_at_order_id;
DROP INDEX IF EXISTS idx_orders_status_completed_at;
DROP INDEX IF EXISTS idx_orders_completed_at_desc;
-- +goose StatementEnd
