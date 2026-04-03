-- +goose Up
-- +goose StatementBegin

ALTER TABLE public.users
    ADD COLUMN IF NOT EXISTS is_head BOOLEAN NOT NULL DEFAULT FALSE;

COMMENT ON COLUMN public.users.is_head IS 'Является ли пользователь руководителем своего департамента (True/False)';

-- +goose StatementEnd

-- +goose Down
-- +goose StatementBegin

ALTER TABLE public.users 
    DROP COLUMN IF EXISTS is_head;

-- +goose StatementEnd
