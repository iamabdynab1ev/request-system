-- +goose Up
-- +goose StatementBegin
SELECT 'up: optimizing dashboard history event indexes';

-- KPI и SLA теперь опираются на историю переходов в CLOSED.
CREATE INDEX IF NOT EXISTS idx_order_history_status_change_order_new_value_created_at
    ON public.order_history (order_id, new_value, created_at DESC)
    WHERE event_type = 'STATUS_CHANGE';

-- Active agents ищет последнее назначение исполнителя на момент действия.
CREATE INDEX IF NOT EXISTS idx_order_history_delegation_order_created_at
    ON public.order_history (order_id, created_at DESC)
    WHERE event_type = 'DELEGATION';

-- Для period-based active_agents полезно иметь компактный индекс по рабочим событиям.
CREATE INDEX IF NOT EXISTS idx_order_history_activity_created_at_order_user
    ON public.order_history (created_at DESC, order_id, user_id)
    WHERE user_id IS NOT NULL
      AND event_type <> 'CREATE'
      AND event_type <> 'DELEGATION';
-- +goose StatementEnd

-- +goose Down
-- +goose StatementBegin
SELECT 'down: removing dashboard history event indexes';

DROP INDEX IF EXISTS idx_order_history_activity_created_at_order_user;
DROP INDEX IF EXISTS idx_order_history_delegation_order_created_at;
DROP INDEX IF EXISTS idx_order_history_status_change_order_new_value_created_at;
-- +goose StatementEnd
