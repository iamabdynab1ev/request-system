-- +goose Up
-- +goose StatementBegin
CREATE TABLE equipment_types (
    id SERIAL PRIMARY KEY,
    name VARCHAR(255) NOT NULL,
  
    created_at TIMESTAMP DEFAULT CURRENT_TIMESTAMP,
    updated_at TIMESTAMP DEFAULT CURRENT_TIMESTAMP
);
-- +goose StatementEnd

-- +goose Down
-- +goose StatementBegin
DROP TABLE IF EXISTS equipment_types;
-- +goose StatementEnd

