-- +goose Up
-- +goose StatementBegin
SELECT 'up: backfilling completed metrics from order history';

WITH latest_completed AS (
    SELECT DISTINCT ON (h.order_id)
        h.order_id,
        h.created_at AS completed_at
    FROM public.order_history h
    JOIN public.statuses completed_status
        ON completed_status.code = 'COMPLETED'
    WHERE h.event_type = 'STATUS_CHANGE'
      AND h.new_value = completed_status.id::text
    ORDER BY h.order_id, h.created_at DESC
),
backfill AS (
    SELECT
        o.id,
        lc.completed_at,
        GREATEST(EXTRACT(EPOCH FROM (lc.completed_at - o.created_at)), 0)::bigint AS resolution_seconds
    FROM public.orders o
    JOIN latest_completed lc
        ON lc.order_id = o.id
    JOIN public.statuses current_status
        ON current_status.id = o.status_id
    WHERE current_status.code IN ('COMPLETED', 'CLOSED')
      AND (
          o.completed_at IS NULL
          OR o.resolution_time_seconds IS NULL
          OR o.is_first_contact_resolution IS NULL
      )
)
UPDATE public.orders o
SET
    completed_at = COALESCE(o.completed_at, b.completed_at),
    resolution_time_seconds = COALESCE(o.resolution_time_seconds, b.resolution_seconds),
    is_first_contact_resolution = COALESCE(o.is_first_contact_resolution, b.resolution_seconds <= 600)
FROM backfill b
WHERE o.id = b.id;
-- +goose StatementEnd

-- +goose Down
-- +goose StatementBegin
SELECT 'down: no-op for completed metrics backfill';
-- +goose StatementEnd
