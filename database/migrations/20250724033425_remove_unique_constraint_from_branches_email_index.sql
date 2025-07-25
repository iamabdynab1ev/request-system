-- +goose Up
-- +goose StatementBegin
-- Удаляем ограничение уникальности с колонки email_index в таблице branches.
-- Имя ограничения 'branches_email_index_key' взято из вашей ошибки.
ALTER TABLE branches DROP CONSTRAINT branches_email_index_key;
-- +goose StatementEnd

-- +goose Down
-- +goose StatementBegin
-- Код для отката: возвращает ограничение уникальности обратно.
ALTER TABLE branches ADD CONSTRAINT branches_email_index_key UNIQUE (email_index);
-- +goose StatementEnd