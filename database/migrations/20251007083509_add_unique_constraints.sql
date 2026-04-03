-- +goose Up
-- +goose StatementBegin

-- Идемпотентное добавление UNIQUE ограничений (с проверкой на существование)

-- Для таблицы departments
DO $$
BEGIN
    IF NOT EXISTS (SELECT 1 FROM pg_constraint WHERE conname = 'departments_name_unique') THEN
        ALTER TABLE departments ADD CONSTRAINT departments_name_unique UNIQUE (name);
    END IF;
END$$;

-- Для таблицы branches
DO $$
BEGIN
    IF NOT EXISTS (SELECT 1 FROM pg_constraint WHERE conname = 'branches_name_unique') THEN
        ALTER TABLE branches ADD CONSTRAINT branches_name_unique UNIQUE (name);
    END IF;
END$$;

-- Для таблицы offices
DO $$
BEGIN
    IF NOT EXISTS (SELECT 1 FROM pg_constraint WHERE conname = 'offices_name_unique') THEN
        ALTER TABLE offices ADD CONSTRAINT offices_name_unique UNIQUE (name);
    END IF;
END$$;

-- Для таблицы otdels (если она есть, раскомментируйте блок)
/*
DO $$
BEGIN
    IF EXISTS (SELECT 1 FROM information_schema.tables WHERE table_name = 'otdels')
    AND NOT EXISTS (SELECT 1 FROM pg_constraint WHERE conname = 'otdels_name_unique') THEN
        ALTER TABLE otdels ADD CONSTRAINT otdels_name_unique UNIQUE (name);
    END IF;
END$$;
*/

-- Для таблицы positions (name)
DO $$
BEGIN
    IF NOT EXISTS (SELECT 1 FROM pg_constraint WHERE conname = 'positions_name_unique') THEN
        ALTER TABLE positions ADD CONSTRAINT positions_name_unique UNIQUE (name);
    END IF;
END$$;

-- Для таблицы positions (code)
DO $$
BEGIN
    IF NOT EXISTS (SELECT 1 FROM pg_constraint WHERE conname = 'positions_code_unique') THEN
        ALTER TABLE positions ADD CONSTRAINT positions_code_unique UNIQUE (code);
    END IF;
END$$;

-- Для таблицы statuses (code)
DO $$
BEGIN
    IF NOT EXISTS (SELECT 1 FROM pg_constraint WHERE conname = 'statuses_code_unique') THEN
        ALTER TABLE statuses ADD CONSTRAINT statuses_code_unique UNIQUE (code);
    END IF;
END$$;

-- Для таблицы priorities (code)
DO $$
BEGIN
    IF NOT EXISTS (SELECT 1 FROM pg_constraint WHERE conname = 'priorities_code_unique') THEN
        ALTER TABLE priorities ADD CONSTRAINT priorities_code_unique UNIQUE (code);
    END IF;
END$$;

SELECT 'Finished adding unique constraints (idempotent)';

-- +goose StatementEnd


-- +goose Down
-- +goose StatementBegin

-- Откатываем добавленные UNIQUE ограничения

ALTER TABLE departments DROP CONSTRAINT IF EXISTS departments_name_unique;
ALTER TABLE branches DROP CONSTRAINT IF EXISTS branches_name_unique;
ALTER TABLE offices DROP CONSTRAINT IF EXISTS offices_name_unique;
ALTER TABLE otdels DROP CONSTRAINT IF EXISTS otdels_name_unique;
ALTER TABLE positions DROP CONSTRAINT IF EXISTS positions_name_unique;
ALTER TABLE positions DROP CONSTRAINT IF EXISTS positions_code_unique;
ALTER TABLE statuses DROP CONSTRAINT IF EXISTS statuses_code_unique;
ALTER TABLE priorities DROP CONSTRAINT IF EXISTS priorities_code_unique;

SELECT 'Finished dropping unique constraints';

-- +goose StatementEnd