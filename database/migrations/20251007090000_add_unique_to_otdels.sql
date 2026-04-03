-- +goose Up
-- +goose StatementBegin

-- Добавляем UNIQUE ограничение для названия отдела
ALTER TABLE otdels ADD CONSTRAINT otdels_name_unique UNIQUE (name);

SELECT 'Added unique constraint to otdels table';

-- +goose StatementEnd


-- +goose Down
-- +goose StatementBegin

-- Откатываем изменение
ALTER TABLE otdels DROP CONSTRAINT IF EXISTS otdels_name_unique;

SELECT 'Dropped unique constraint from otdels table';

-- +goose StatementEnd