-- +goose Up
-- +goose StatementBegin
SELECT 'up: recomputing FCR from recorded response/resolution metrics';

UPDATE public.orders o
SET is_first_contact_resolution = CASE
    WHEN o.first_response_time_seconds IS NULL OR o.resolution_time_seconds IS NULL THEN false
    WHEN o.first_response_time_seconds = o.resolution_time_seconds THEN true
    ELSE false
END
FROM public.statuses s
WHERE s.id = o.status_id
  AND s.code IN ('COMPLETED', 'CLOSED')
  AND o.deleted_at IS NULL
  AND o.is_first_contact_resolution IS DISTINCT FROM CASE
        WHEN o.first_response_time_seconds IS NULL OR o.resolution_time_seconds IS NULL THEN false
        WHEN o.first_response_time_seconds = o.resolution_time_seconds THEN true
        ELSE false
      END;
-- +goose StatementEnd

-- +goose Down
-- +goose StatementBegin
SELECT 'down: no-op for FCR recompute';
-- +goose StatementEnd
