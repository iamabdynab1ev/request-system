-- +goose Up
-- +goose StatementBegin

-- Этап 1: Удаляем колонки с иконками
ALTER TABLE priorities
    DROP COLUMN IF EXISTS icon_small,
    DROP COLUMN IF EXISTS icon_big;

-- Этап 2: Делаем колонку 'code' необязательной (разрешаем NULL)
-- Это позволит фронтенду не отправлять 'code', а бэкенду - сгенерировать его.
ALTER TABLE priorities
    ALTER COLUMN code DROP NOT NULL;

-- +goose StatementEnd

-- +goose Down
-- +goose StatementBegin

-- Этот блок нужен для отката миграции, если что-то пойдет не так.
-- Он вернет колонки и сделает 'code' снова обязательным.
ALTER TABLE priorities
    ADD COLUMN icon_small VARCHAR(100),
    ADD COLUMN icon_big VARCHAR(100);

ALTER TABLE priorities
    ALTER COLUMN code SET NOT NULL;

-- +goose StatementEnd