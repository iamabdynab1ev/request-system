-- +goose Up
-- +goose StatementBegin
SELECT 'up SQL query';

-- Добавляем колонку, ТОЛЬКО ЕСЛИ она еще не существует
ALTER TABLE orders ADD COLUMN IF NOT EXISTS order_type_id INTEGER;

-- Добавляем внешний ключ для связи с таблицей order_types
ALTER TABLE orders DROP CONSTRAINT IF EXISTS fk_orders_order_type_id;
ALTER TABLE orders ADD CONSTRAINT fk_orders_order_type_id 
    FOREIGN KEY (order_type_id) 
    REFERENCES order_types(id) 
    ON DELETE SET NULL;

-- +goose StatementEnd

-- +goose Down
-- +goose StatementBegin
SELECT 'down SQL query';

-- Откатываем изменения
ALTER TABLE orders DROP CONSTRAINT IF EXISTS fk_orders_order_type_id;
ALTER TABLE orders DROP COLUMN IF EXISTS order_type_id;

-- +goose StatementEnd
