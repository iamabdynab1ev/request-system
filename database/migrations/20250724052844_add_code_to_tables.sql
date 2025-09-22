-- +goose Up
-- SQL in this section is executed when the migration is applied.
-- Эта миграция больше не нужна, так как таблицы statuses и priorities
-- уже создаются с колонкой 'code'.
-- Оставляем её пустой.

-- +goose Down
-- SQL in this section is executed when the migration is rolled back.