-- +goose Up
-- +goose StatementBegin
SELECT 'up: adding unique constraints for seeder tables';

-- Добавляем UNIQUE ограничение на имя в таблице equipment_types, если его еще нет.
DO $$
BEGIN
    IF NOT EXISTS (
        SELECT 1 FROM pg_constraint 
        WHERE conname = 'equipment_types_name_unique' AND conrelid = 'equipment_types'::regclass
    ) THEN
        ALTER TABLE public.equipment_types ADD CONSTRAINT equipment_types_name_unique UNIQUE (name);
    END IF;
END $$;

-- Добавляем UNIQUE ограничение на код в таблице order_types, если его еще нет.
DO $$
BEGIN
    IF NOT EXISTS (
        SELECT 1 FROM pg_constraint 
        WHERE conname = 'order_types_code_unique' AND conrelid = 'order_types'::regclass
    ) THEN
        ALTER TABLE public.order_types ADD CONSTRAINT order_types_code_unique UNIQUE (code);
    END IF;
END $$;

-- +goose StatementEnd

-- +goose Down
-- +goose StatementBegin
-- ... (секция Down остается без изменений)
SELECT 'down: removing unique constraints from seeder tables';

ALTER TABLE public.equipment_types DROP CONSTRAINT IF EXISTS equipment_types_name_unique;
ALTER TABLE public.order_types DROP CONSTRAINT IF EXISTS order_types_code_unique;

-- +goose StatementEnd

