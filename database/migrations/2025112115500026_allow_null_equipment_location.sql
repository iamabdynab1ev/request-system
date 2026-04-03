-- +goose Up
-- +goose StatementBegin
ALTER TABLE public.equipments ALTER COLUMN branch_id DROP NOT NULL;

ALTER TABLE public.equipments ALTER COLUMN office_id DROP NOT NULL;

-- +goose StatementEnd

-- +goose Down
-- +goose StatementBegin


ALTER TABLE public.equipments ALTER COLUMN branch_id SET NOT NULL;
ALTER TABLE public.equipments ALTER COLUMN office_id SET NOT NULL;

-- +goose StatementEnd

