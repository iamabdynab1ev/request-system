-- +goose Up
-- +goose StatementBegin

-- --- Изменяем таблицу statuses ---
-- 1. Добавляем новую колонку 'code'
ALTER TABLE statuses ADD COLUMN code VARCHAR(50);
-- 2. Делаем её уникальной, так как коды не должны повторяться
ALTER TABLE statuses ADD CONSTRAINT statuses_code_unique UNIQUE (code);
-- 3. Заполняем код для статуса 'Открыто'
UPDATE statuses SET code = 'OPEN' WHERE name = 'Открыто';


-- --- Изменяем таблицу priorities ---
-- 1. Добавляем новую колонку 'code'
ALTER TABLE priorities ADD COLUMN code VARCHAR(50);
-- 2. Делаем её уникальной
ALTER TABLE priorities ADD CONSTRAINT priorities_code_unique UNIQUE (code);
-- 3. Заполняем код для приоритета 'Средний'
UPDATE priorities SET code = 'MEDIUM' WHERE name = 'Средний';

-- +goose StatementEnd

-- +goose Down
-- +goose StatementBegin
-- Код для отката изменений: удаляем колонку 'code' из обеих таблиц
ALTER TABLE priorities DROP CONSTRAINT priorities_code_unique;
ALTER TABLE priorities DROP COLUMN code;

ALTER TABLE statuses DROP CONSTRAINT statuses_code_unique;
ALTER TABLE statuses DROP COLUMN code;
-- +goose StatementEnd