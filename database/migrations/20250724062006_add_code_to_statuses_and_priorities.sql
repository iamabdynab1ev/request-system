-- +goose Up
-- +goose StatementBegin

-- --- Изменяем таблицу statuses ---
ALTER TABLE statuses ADD COLUMN IF NOT EXISTS code VARCHAR(50);
UPDATE statuses SET code = 'OPEN' WHERE name = 'Открыто';
UPDATE statuses SET code = 'CLOSED' WHERE name = 'Закрыто';
UPDATE statuses SET code = 'COMPLETED' WHERE name = 'Выполнено';
UPDATE statuses SET code = 'IN_PROGRESS' WHERE name = 'В работе';
UPDATE statuses SET code = 'REJECTED' WHERE name = 'Отклонить';
UPDATE statuses SET code = 'CLARIFICATION' WHERE name = 'Уточнение';
UPDATE statuses SET code = 'REFINEMENT' WHERE name = 'Доработка';
UPDATE statuses SET code = 'CONFIRMED' WHERE name = 'Подтверждён';
UPDATE statuses SET code = 'SERVICE' WHERE name = 'Сервис';

-- --- Изменяем таблицу priorities ---
ALTER TABLE priorities ADD COLUMN IF NOT EXISTS code VARCHAR(50);
UPDATE priorities SET code = 'CRITICAL' WHERE name = 'Критический';
UPDATE priorities SET code = 'HIGH' WHERE name = 'Высокий';
UPDATE priorities SET code = 'MEDIUM' WHERE name = 'Средний';
UPDATE priorities SET code = 'LOW' WHERE name = 'Низкий';

-- +goose StatementEnd

-- +goose Down
-- +goose StatementBegin
ALTER TABLE priorities DROP COLUMN IF EXISTS code;
ALTER TABLE statuses DROP COLUMN IF EXISTS code;
-- +goose StatementEnd