-- +goose Up
-- SQL in this section is executed when the migration is applied.
-- Эта миграция больше не нужна, так как таблица statuses
-- уже создается с колонками icon_small и icon_big.
-- Оставляем её пустой.

-- +goose Down
-- SQL in this section is executed when the migration is rolled back.