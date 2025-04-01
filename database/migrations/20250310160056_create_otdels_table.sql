-- +goose Up
-- +goose StatementBegin
CREATE TABLE otdels (
    id SERIAL PRIMARY KEY,
    name VARCHAR(255) NOT NULL,
    status_id INT NOT NULL,
    departments_id INT NOT NULL,

    created_at TIMESTAMP DEFAULT CURRENT_TIMESTAMP,
    updated_at TIMESTAMP DEFAULT CURRENT_TIMESTAMP,

    CONSTRAINT fk_otdel_status FOREIGN KEY (status_id) REFERENCES statuses(id),
    CONSTRAINT fk_otdel_departments FOREIGN KEY (departments_id) REFERENCES departments(id)
);
-- +goose StatementEnd

-- +goose Down
-- +goose StatementBegin
DROP TABLE IF EXISTS otdels;
-- +goose StatementEnd
