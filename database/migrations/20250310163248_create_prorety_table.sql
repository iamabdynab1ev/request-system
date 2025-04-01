-- +goose Up
-- +goose StatementBegin
CREATE TABLE proreties (
    id SERIAL PRIMARY KEY,
    icon VARCHAR(255) NOT NULL,
    name VARCHAR(255) NOT NULL,
    rate INT NOT NULL,
    created_at TIMESTAMP DEFAULT CURRENT_TIMESTAMP,
    updated_at TIMESTAMP DEFAULT CURRENT_TIMESTAMP
);
-- +goose StatementEnd

-- +goose Down
-- +goose StatementBegin
DROP TABLE IF EXISTS proreties;
-- +goose StatementEnd

