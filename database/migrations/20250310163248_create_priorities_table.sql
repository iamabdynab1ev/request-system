-- +goose Up
-- +goose StatementBegin
CREATE TABLE priorities (
    id BIGSERIAL PRIMARY KEY,
    icon_small VARCHAR(100), 
    icon_big VARCHAR(100),   
    name VARCHAR(255) NOT NULL,
    rate BIGINT NOT NULL,
    code VARCHAR(50) UNIQUE NOT NULL,
    created_at TIMESTAMPTZ DEFAULT NOW(),
    updated_at TIMESTAMPTZ DEFAULT NOW(),
    deleted_at TIMESTAMPTZ
);
-- +goose StatementEnd

-- +goose Down
-- +goose StatementBegin
DROP TABLE IF EXISTS priorities;
-- +goose StatementEnd