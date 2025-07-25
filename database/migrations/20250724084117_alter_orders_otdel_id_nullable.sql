-- +goose Up
-- +goose StatementBegin
-- Делаем ВСЕ необязательные поля в таблице 'orders' разрешающими NULL, чтобы закончить это раз и навсегда
ALTER TABLE orders ALTER COLUMN otdel_id DROP NOT NULL;
ALTER TABLE orders ALTER COLUMN branch_id DROP NOT NULL;
ALTER TABLE orders ALTER COLUMN office_id DROP NOT NULL;
ALTER TABLE orders ALTER COLUMN equipment_id DROP NOT NULL;
ALTER TABLE orders ALTER COLUMN duration DROP NOT NULL;
-- +goose StatementEnd

-- +goose Down
-- +goose StatementBegin
-- Код для отката: возвращаем ограничения NOT NULL
ALTER TABLE orders ALTER COLUMN otdel_id SET NOT NULL;
ALTER TABLE orders ALTER COLUMN branch_id SET NOT NULL;
ALTER TABLE orders ALTER COLUMN office_id SET NOT NULL;
ALTER TABLE orders ALTER COLUMN equipment_id SET NOT NULL;
ALTER TABLE orders ALTER COLUMN duration SET NOT NULL;
-- +goose StatementEnd