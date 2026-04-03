-- +goose Up
-- +goose StatementBegin

-- Убираем ограничение NOT NULL с колонки address в таблице orders
ALTER TABLE public.orders
    ALTER COLUMN address DROP NOT NULL;

-- +goose StatementEnd

-- +goose Down
-- +goose StatementBegin

-- Этот блок нужен для отката. Он вернет ограничение обратно.
-- ВАЖНО: Если в таблице уже будут строки с address = NULL,
-- эта команда (откат) выдаст ошибку.
ALTER TABLE public.orders
    ALTER COLUMN address SET NOT NULL;

-- +goose StatementEnd
