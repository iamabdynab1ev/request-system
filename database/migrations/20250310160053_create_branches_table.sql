-- +goose Up
-- +goose StatementBegin
CREATE TABLE branches (
    id SERIAL PRIMARY KEY,
    name VARCHAR(255) NOT NULL,
    short_name VARCHAR(255) NOT NULL,
    address TEXT NOT NULL,
    phone_number VARCHAR(12) NOT NULL,
    email VARCHAR(255) NOT NULL,
    email_index VARCHAR(255) UNIQUE,
    open_date TIMESTAMP NOT NULL,
    status_id INTEGER NOT NULL,

    created_at TIMESTAMP WITH TIME ZONE DEFAULT CURRENT_TIMESTAMP,
    updated_at TIMESTAMP WITH TIME ZONE DEFAULT CURRENT_TIMESTAMP,

    CONSTRAINT fk_branches_status_id FOREIGN KEY (status_id) REFERENCES statuses(id) ON DELETE RESTRICT
);
-- +goose StatementEnd

-- +goose Down
-- +goose StatementBegin
DROP TABLE IF EXISTS branches;
-- +goose StatementEnd