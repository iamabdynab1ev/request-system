-- +goose Up
-- +goose StatementBegin
SELECT 'up: making department_id and branch_id nullable in orders';
ALTER TABLE public.orders ALTER COLUMN department_id DROP NOT NULL;
ALTER TABLE public.orders ALTER COLUMN branch_id DROP NOT NULL;
-- +goose StatementEnd

-- +goose Down
-- +goose StatementBegin
SELECT 'down: reverting department_id to NOT NULL';
UPDATE public.orders SET department_id = 1 WHERE department_id IS NULL; 
ALTER TABLE public.orders ALTER COLUMN department_id SET NOT NULL;

-- +goose StatementEnd

