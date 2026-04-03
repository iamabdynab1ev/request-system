-- +goose Up
-- +goose StatementBegin
SELECT 'up SQL query';

-- =============================================================================
-- ШАГ 1: СОЗДАНИЕ НОВЫХ ТАБЛИЦ (Версия MVP)
-- =============================================================================

CREATE TABLE IF NOT EXISTS order_types (
    id SERIAL PRIMARY KEY,
    name VARCHAR(255) NOT NULL,
    code VARCHAR(100) UNIQUE,
    status_id INTEGER,
    created_at TIMESTAMP DEFAULT CURRENT_TIMESTAMP,
    updated_at TIMESTAMP DEFAULT CURRENT_TIMESTAMP
);

CREATE TABLE IF NOT EXISTS order_routing_rules (
    id SERIAL PRIMARY KEY,
    rule_name VARCHAR(255) NOT NULL,
    order_type_id INTEGER REFERENCES order_types(id) ON DELETE SET NULL,
    department_id INTEGER,
    branch_id INTEGER,
    office_id INTEGER,
    otdel_id INTEGER,
    assign_to_position_id INTEGER REFERENCES positions(id) ON DELETE SET NULL,
    status_id INTEGER,
    created_at TIMESTAMP DEFAULT CURRENT_TIMESTAMP,
    updated_at TIMESTAMP DEFAULT CURRENT_TIMESTAMP
);

-- =============================================================================
-- ШАГ 2: ОБНОВЛЕНИЕ СУЩЕСТВУЮЩИХ ТАБЛИЦ (positions и users)
-- =============================================================================

ALTER TABLE positions ADD COLUMN IF NOT EXISTS code VARCHAR(100) UNIQUE;
ALTER TABLE positions ADD COLUMN IF NOT EXISTS level INTEGER NOT NULL DEFAULT 0;
ALTER TABLE positions ADD COLUMN IF NOT EXISTS status_id INTEGER;

ALTER TABLE users ADD COLUMN IF NOT EXISTS position_id INTEGER;
ALTER TABLE users DROP COLUMN IF EXISTS "position";

-- Пересоздаем constraint для надежности
ALTER TABLE users DROP CONSTRAINT IF EXISTS fk_users_position_id;
ALTER TABLE users ADD CONSTRAINT fk_users_position_id FOREIGN KEY (position_id) REFERENCES positions(id) ON DELETE SET NULL;


-- +goose StatementEnd

-- +goose Down
-- +goose StatementBegin
SELECT 'down SQL query';

-- Откат
ALTER TABLE users ADD COLUMN IF NOT EXISTS "position" VARCHAR(255);
ALTER TABLE users DROP CONSTRAINT IF EXISTS fk_users_position_id;
ALTER TABLE users DROP COLUMN IF EXISTS position_id;


ALTER TABLE positions DROP COLUMN IF EXISTS code;
ALTER TABLE positions DROP COLUMN IF EXISTS level;
ALTER TABLE positions DROP COLUMN IF EXISTS status_id;

DROP TABLE IF EXISTS order_routing_rules;
DROP TABLE IF EXISTS order_types;

-- +goose StatementEnd
