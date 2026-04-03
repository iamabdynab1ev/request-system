-- +goose Up
-- +goose StatementBegin

DO $$
DECLARE
    r RECORD;
BEGIN
    
    FOR r IN 
        SELECT 
            table_name, 
            column_name
        FROM information_schema.columns
        WHERE table_schema = 'public'
          AND data_type = 'timestamp without time zone'
          AND column_name IN ('created_at', 'updated_at', 'deleted_at')
          AND table_name IN (
              'orders', 'users', 'order_history', 'attachments',
              'departments', 'otdels', 'branches', 'offices',
              'equipments', 'equipment_types', 'statuses', 'priorities',
              'order_types', 'positions', 'roles', 'permissions', 'order_rules'
          )
    LOOP
        -- Изменяем тип колонки
        EXECUTE format(
            'ALTER TABLE %I ALTER COLUMN %I TYPE timestamptz USING %I AT TIME ZONE ''Asia/Tashkent''',
            r.table_name, 
            r.column_name, 
            r.column_name
        );
        
        RAISE NOTICE 'Изменён тип: %.% -> timestamptz', r.table_name, r.column_name;
    END LOOP;
END $$;

-- +goose StatementEnd

-- +goose Down
-- +goose StatementBegin

DO $$
DECLARE
    r RECORD;
BEGIN
    FOR r IN 
        SELECT 
            table_name, 
            column_name
        FROM information_schema.columns
        WHERE table_schema = 'public'
          AND data_type = 'timestamp with time zone'
          AND column_name IN ('created_at', 'updated_at', 'deleted_at')
          AND table_name IN (
              'orders', 'users', 'order_history', 'attachments',
              'departments', 'otdels', 'branches', 'offices',
              'equipments', 'equipment_types', 'statuses', 'priorities',
              'order_types', 'positions', 'roles', 'permissions', 'order_rules'
          )
    LOOP
        EXECUTE format(
            'ALTER TABLE %I ALTER COLUMN %I TYPE timestamp without time zone USING %I AT TIME ZONE ''Asia/Tashkent''',
            r.table_name, 
            r.column_name, 
            r.column_name
        );
        
        RAISE NOTICE 'Откат: %.% -> timestamp without time zone', r.table_name, r.column_name;
    END LOOP;
END $$;

-- +goose StatementEnd
