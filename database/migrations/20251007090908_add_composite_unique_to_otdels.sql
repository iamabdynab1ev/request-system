-- +goose Up
-- +goose StatementBegin

-- Сначала удаляем старое, неправильное ограничение, если оно есть
ALTER TABLE otdels DROP CONSTRAINT IF EXISTS otdels_name_unique;

-- Теперь создаем новое, правильное составное ограничение
ALTER TABLE otdels ADD CONSTRAINT otdels_name_department_id_unique UNIQUE (name, department_id);

SELECT 'Replaced unique constraint for otdels table';

-- +goose StatementEnd


-- +goose Down
-- +goose StatementBegin

-- Откатываем изменения в обратном порядке
ALTER TABLE otdels DROP CONSTRAINT IF EXISTS otdels_name_department_id_unique;
ALTER TABLE otdels ADD CONSTRAINT otdels_name_unique UNIQUE (name);

SELECT 'Restored old unique constraint for otdels table';

-- +goose StatementEnd

