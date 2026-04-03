-- +goose Up
-- +goose StatementBegin

-- 1. ВКЛЮЧАЕМ УМНЫЙ ПОИСК (pg_trgm)
-- Если расширение уже есть, команда просто пропустится без ошибки.
CREATE EXTENSION IF NOT EXISTS pg_trgm;

-- 2. УСКОРЯЕМ ПОИСК ПО ТЕКСТУ (Название и Адрес заявки)
-- Индексы GIN позволяют искать '%текст%' за миллисекунды.
CREATE INDEX IF NOT EXISTS idx_orders_name_trgm ON orders USING gin (name gin_trgm_ops);
CREATE INDEX IF NOT EXISTS idx_orders_address_trgm ON orders USING gin (address gin_trgm_ops);

-- 3. УСКОРЯЕМ СПИСКИ И ФИЛЬТРЫ (Внешние ключи)
-- Без этих индексов фильтрация "По отделу" или "По статусу" сканирует всю таблицу.
CREATE INDEX IF NOT EXISTS idx_orders_user_id ON orders(user_id);
CREATE INDEX IF NOT EXISTS idx_orders_executor_id ON orders(executor_id);
CREATE INDEX IF NOT EXISTS idx_orders_status_id ON orders(status_id);
CREATE INDEX IF NOT EXISTS idx_orders_priority_id ON orders(priority_id);

-- Индексы для орг. структуры (отделы, филиалы и т.д.)
CREATE INDEX IF NOT EXISTS idx_orders_department_id ON orders(department_id);
CREATE INDEX IF NOT EXISTS idx_orders_branch_id ON orders(branch_id);
CREATE INDEX IF NOT EXISTS idx_orders_otdel_id ON orders(otdel_id);
CREATE INDEX IF NOT EXISTS idx_orders_office_id ON orders(office_id);

-- 4. УСКОРЯЕМ ДАШБОРД (Составные индексы)
-- Самые частые запросы аналитики.
-- Покрывают: "Заявки Васи за период" и "Заявки в статусе Х за период".
CREATE INDEX IF NOT EXISTS idx_orders_user_created ON orders(user_id, created_at);
CREATE INDEX IF NOT EXISTS idx_orders_executor_created ON orders(executor_id, created_at);
CREATE INDEX IF NOT EXISTS idx_orders_status_created ON orders(status_id, created_at);

-- 5. УСКОРЯЕМ ОБЩУЮ СОРТИРОВКУ
-- Чтобы список "Все заявки" (от новых к старым) открывался мгновенно без сортировки в памяти.
CREATE INDEX IF NOT EXISTS idx_orders_created_at_desc ON orders(created_at DESC);

-- +goose StatementEnd

-- +goose Down
-- +goose StatementBegin
-- Удаление индексов при откате миграции
DROP INDEX IF EXISTS idx_orders_created_at_desc;
DROP INDEX IF EXISTS idx_orders_status_created;
DROP INDEX IF EXISTS idx_orders_executor_created;
DROP INDEX IF EXISTS idx_orders_user_created;
DROP INDEX IF EXISTS idx_orders_office_id;
DROP INDEX IF EXISTS idx_orders_otdel_id;
DROP INDEX IF EXISTS idx_orders_branch_id;
DROP INDEX IF EXISTS idx_orders_department_id;
DROP INDEX IF EXISTS idx_orders_priority_id;
DROP INDEX IF EXISTS idx_orders_status_id;
DROP INDEX IF EXISTS idx_orders_executor_id;
DROP INDEX IF EXISTS idx_orders_user_id;
DROP INDEX IF EXISTS idx_orders_address_trgm;
DROP INDEX IF EXISTS idx_orders_name_trgm;
-- +goose StatementEnd



