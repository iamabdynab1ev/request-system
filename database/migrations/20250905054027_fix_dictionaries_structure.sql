-- +goose Up
-- +goose StatementBegin

-- ШАГ 1: Делаем названия уникальными, чтобы работал ON CONFLICT в сидерах.
-- Это предотвратит создание дубликатов.
ALTER TABLE public.branches ADD CONSTRAINT branches_name_unique UNIQUE (name);
ALTER TABLE public.departments ADD CONSTRAINT departments_name_unique UNIQUE (name);

-- ШАГ 2: Делаем все поля в 'branches', кроме основных, НЕОБЯЗАТЕЛЬНЫМИ.
-- Это позволяет сидеру создавать базовый филиал, не зная всех деталей.
ALTER TABLE public.branches ALTER COLUMN address DROP NOT NULL;
ALTER TABLE public.branches ALTER COLUMN phone_number DROP NOT NULL;
ALTER TABLE public.branches ALTER COLUMN email DROP NOT NULL;
ALTER TABLE public.branches ALTER COLUMN email_index DROP NOT NULL;
ALTER TABLE public.branches ALTER COLUMN open_date DROP NOT NULL;

-- +goose StatementEnd

-- +goose Down
-- +goose StatementBegin

-- Возвращаем всё как было, если нужно откатить.
ALTER TABLE public.branches DROP CONSTRAINT branches_name_unique;
ALTER TABLE public.departments DROP CONSTRAINT departments_name_unique;

-- Обратно делаем поля ОБЯЗАТЕЛЬНЫМИ (возвращаем старую, строгую структуру).
ALTER TABLE public.branches ALTER COLUMN address SET NOT NULL;
ALTER TABLE public.branches ALTER COLUMN phone_number SET NOT NULL;
ALTER TABLE public.branches ALTER COLUMN email SET NOT NULL;
ALTER TABLE public.branches ALTER COLUMN email_index SET NOT NULL;
ALTER TABLE public.branches ALTER COLUMN open_date SET NOT NULL;

-- +goose StatementEnd

