-- +goose Up
-- +goose StatementBegin
-- Изменяем таблицу priorities: удаляем старую колонку и добавляем две новые
ALTER TABLE priorities
    DROP COLUMN icon,
    ADD COLUMN icon_small VARCHAR(100) NOT NULL,
    ADD COLUMN icon_big VARCHAR(100) NOT NULL;
-- +goose StatementEnd

-- +goose Down
-- +goose StatementBegin
-- Код для отката изменений: удаляем новые колонки и возвращаем старую
ALTER TABLE priorities
    DROP COLUMN icon_small,
    DROP COLUMN icon_big,
    ADD COLUMN icon VARCHAR(255) NOT NULL;
-- +goose StatementEnd