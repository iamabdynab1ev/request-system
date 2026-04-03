-- +goose Up
-- +goose StatementBegin
SELECT 'up: changing resolution_time columns to BIGINT in orders table';

ALTER TABLE public.orders
    ALTER COLUMN resolution_time_seconds TYPE BIGINT,
    ALTER COLUMN first_response_time_seconds TYPE BIGINT;

-- +goose StatementEnd

-- +goose Down
-- +goose StatementBegin
SELECT 'down: changing resolution_time columns back to INTEGER in orders table';

ALTER TABLE public.orders
    ALTER COLUMN resolution_time_seconds TYPE INTEGER,
    ALTER COLUMN first_response_time_seconds TYPE INTEGER;

-- +goose StatementEnd

