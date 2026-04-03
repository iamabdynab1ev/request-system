-- +goose Up
-- +goose StatementBegin
-- Добавляем уникальность для имени оборудования, чтобы работал UPSERT (обновление при дубликатах)
CREATE UNIQUE INDEX idx_equipments_name_unique ON public.equipments(name);
-- +goose StatementEnd

-- +goose Down
-- +goose StatementBegin
DROP INDEX IF EXISTS idx_equipments_name_unique;
-- +goose StatementEnd



