-- +goose Up
-- +goose StatementBegin
CREATE TABLE offices (
    id SERIAL PRIMARY KEY,
    name VARCHAR(255) NOT NULL,
    address TEXT NOT NULL,
    open_date DATE NOT NULL,

    branche_id INT NOT NULL,
    status_id INT NOT NULL,

    created_at TIMESTAMP DEFAULT CURRENT_TIMESTAMP,
    updated_at TIMESTAMP DEFAULT CURRENT_TIMESTAMP,

    CONSTRAINT fk_office_branches_id FOREIGN KEY (branche_id) REFERENCES branche(id),
    CONSTRAINT fk_office_status_id FOREIGN KEY (status_id) REFERENCES statuses(id)
);
-- +goose StatementEnd

-- +goose Down
-- +goose StatementBegin
DROP TABLE IF EXISTS offices;
-- +goose StatementEnd

