-- +goose Up
-- +goose StatementBegin

ALTER TABLE public.orders
    ADD COLUMN IF NOT EXISTS equipment_type_id bigint DEFAULT NULL;

-- +goose StatementEnd

-- +goose Down
-- +goose StatementBegin

ALTER TABLE public.orders 
    DROP COLUMN IF EXISTS equipment_type_id;

-- +goose StatementEnd