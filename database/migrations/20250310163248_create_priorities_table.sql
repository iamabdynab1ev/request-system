-- +goose Up
-- +goose StatementBegin
CREATE TABLE priorities (
    id BIGSERIAL PRIMARY KEY,
    icon VARCHAR(255) NOT NULL,
    name VARCHAR(255) NOT NULL,
    rate BIGINT NOT NULL,
    code VARCHAR(50) NOT NULL UNIQUE,
    created_at TIMESTAMPTZ DEFAULT NOW(),
    updated_at TIMESTAMPTZ DEFAULT NOW(),
    deleted_at TIMESTAMPTZ
);
-- +goose StatementEnd

-- +goose Down
-- +goose StatementBegin
DROP TABLE IF EXISTS priorities;
-- +goose StatementEnd