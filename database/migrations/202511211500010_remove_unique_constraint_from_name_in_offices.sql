-- +goose Up
-- +goose StatementBegin
SELECT 'up: removing unique constraint from name in offices table';

ALTER TABLE public.offices DROP CONSTRAINT IF EXISTS offices_name_unique;

SELECT 'up: migration completed successfully, duplicates in name are now allowed';
-- +goose StatementEnd

-- +goose Down
-- +goose StatementBegin
SELECT 'down: reverting migration for offices table';


ALTER TABLE public.offices
ADD CONSTRAINT offices_name_unique UNIQUE (name);

SELECT 'down: migration reverted, unique constraint on name restored';
-- +goose StatementEnd


