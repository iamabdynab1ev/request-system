-- +goose Up
-- +goose StatementBegin
SELECT 'up: adding external_id and source_system columns for integration';

-- Добавляем поля и индексы в таблицу Филиалов (branches)
ALTER TABLE public.branches ADD COLUMN external_id VARCHAR(255) NULL;
ALTER TABLE public.branches ADD COLUMN source_system VARCHAR(50) NULL;
CREATE UNIQUE INDEX IF NOT EXISTS idx_branches_source_external_id ON public.branches (source_system, external_id);

-- Добавляем поля и индексы в таблицу Офисов/ЦБО (offices)
ALTER TABLE public.offices ADD COLUMN external_id VARCHAR(255) NULL;
ALTER TABLE public.offices ADD COLUMN source_system VARCHAR(50) NULL;
CREATE UNIQUE INDEX IF NOT EXISTS idx_offices_source_external_id ON public.offices (source_system, external_id);

-- Добавляем поля и индексы в таблицу Пользователей (users)
ALTER TABLE public.users ADD COLUMN external_id VARCHAR(255) NULL;
ALTER TABLE public.users ADD COLUMN source_system VARCHAR(50) NULL;
CREATE UNIQUE INDEX IF NOT EXISTS idx_users_source_external_id ON public.users (source_system, external_id);

-- Добавляем поля и индексы в таблицу Департаментов (departments)
ALTER TABLE public.departments ADD COLUMN external_id VARCHAR(255) NULL;
ALTER TABLE public.departments ADD COLUMN source_system VARCHAR(50) NULL;
CREATE UNIQUE INDEX IF NOT EXISTS idx_departments_source_external_id ON public.departments (source_system, external_id);

-- Добавляем поля и индексы в таблицу Отделов (otdels)
ALTER TABLE public.otdels ADD COLUMN external_id VARCHAR(255) NULL;
ALTER TABLE public.otdels ADD COLUMN source_system VARCHAR(50) NULL;
CREATE UNIQUE INDEX IF NOT EXISTS idx_otdels_source_external_id ON public.otdels (source_system, external_id);

-- Добавляем поля и индексы в таблицу Должностей (positions)
ALTER TABLE public.positions ADD COLUMN external_id VARCHAR(255) NULL;
ALTER TABLE public.positions ADD COLUMN source_system VARCHAR(50) NULL;
CREATE UNIQUE INDEX IF NOT EXISTS idx_positions_source_external_id ON public.positions (source_system, external_id);

-- +goose StatementEnd

-- +goose Down
-- +goose StatementBegin
SELECT 'down: removing external_id and source_system columns';

ALTER TABLE public.branches DROP COLUMN IF EXISTS external_id;
ALTER TABLE public.branches DROP COLUMN IF EXISTS source_system;

ALTER TABLE public.offices DROP COLUMN IF EXISTS external_id;
ALTER TABLE public.offices DROP COLUMN IF EXISTS source_system;

ALTER TABLE public.users DROP COLUMN IF EXISTS external_id;
ALTER TABLE public.users DROP COLUMN IF EXISTS source_system;

ALTER TABLE public.departments DROP COLUMN IF EXISTS external_id;
ALTER TABLE public.departments DROP COLUMN IF EXISTS source_system;

ALTER TABLE public.otdels DROP COLUMN IF EXISTS external_id;
ALTER TABLE public.otdels DROP COLUMN IF EXISTS source_system;
   
ALTER TABLE public.positions DROP COLUMN IF EXISTS external_id;
ALTER TABLE public.positions DROP COLUMN IF EXISTS source_system;

-- +goose StatementEnd

