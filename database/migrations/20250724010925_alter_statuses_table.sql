-- +goose Up
-- +goose StatementBegin
ALTER TABLE statuses
    DROP COLUMN icon,                 -- Удаляем старую колонку icon
    ADD COLUMN icon_small VARCHAR(100), -- Добавляем новую icon_small
    ADD COLUMN icon_big VARCHAR(100);   -- Добавляем новую icon_big

-- Goose требует, чтобы для NOT NULL полей были значения по умолчанию, если в таблице уже есть данные
-- Так как наша таблица пуста, мы можем сразу установить NOT NULL.
-- Если бы были данные, пришлось бы делать в два шага.
ALTER TABLE statuses
    ALTER COLUMN icon_small SET NOT NULL,
    ALTER COLUMN icon_big SET NOT NULL;
-- +goose StatementEnd

-- +goose Down
-- +goose StatementBegin
-- Код для отката изменений: удаляем новые колонки и возвращаем старую
ALTER TABLE statuses
    DROP COLUMN icon_small,
    DROP COLUMN icon_big,
    ADD COLUMN icon VARCHAR(255);
-- +goose StatementEnd