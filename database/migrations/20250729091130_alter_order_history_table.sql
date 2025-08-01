-- +goose Up
-- +goose StatementBegin
ALTER TABLE order_history ADD COLUMN attachment_id BIGINT NULL;

-- Добавление внешнего ключа (опционально, но рекомендуется для целостности данных)
-- Убедитесь, что таблица 'attachments' существует и 'attachments.id' является BIGINT
ALTER TABLE order_history ADD CONSTRAINT fk_order_history_attachment
    FOREIGN KEY (attachment_id) REFERENCES attachments(id) ON DELETE SET NULL;
-- +goose StatementEnd

-- +goose Down
-- +goose StatementBegin
ALTER TABLE order_history DROP CONSTRAINT IF EXISTS fk_order_history_attachment;
ALTER TABLE order_history DROP COLUMN IF EXISTS attachment_id;
-- +goose StatementEnd

как создать 