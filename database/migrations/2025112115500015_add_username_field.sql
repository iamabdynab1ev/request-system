-- +goose Up
-- +goose StatementBegin
ALTER TABLE public.users ADD COLUMN username VARCHAR(100);


CREATE INDEX idx_users_username ON public.users(username);


CREATE UNIQUE INDEX idx_users_username_unique ON public.users(username) WHERE username IS NOT NULL;
-- +goose StatementEnd

-- +goose Down
-- +goose StatementBegin
DROP INDEX IF EXISTS idx_users_username_unique;
DROP INDEX IF EXISTS idx_users_username;
ALTER TABLE public.users DROP COLUMN username;
-- +goose StatementEnd
