-- +goose Up
-- +goose StatementBegin
CREATE TABLE statuses (
    id SERIAL PRIMARY KEY,
    icon VARCHAR(100) NOT NULL,
    name VARCHAR(50) NOT NULL,
    type INT NOT NULL,

    code VARCHAR(50) NOT NULL UNIQUE, 

    created_at TIMESTAMP WITH TIME ZONE DEFAULT CURRENT_TIMESTAMP,
    updated_at TIMESTAMP WITH TIME ZONE DEFAULT CURRENT_TIMESTAMP,
    deleted_at TIMESTAMP WITH TIME ZONE 
);
-- +goose StatementEnd

-- +goose Down
-- +goose StatementBegin
DROP TABLE IF EXISTS statuses;
-- +goose StatementEnd