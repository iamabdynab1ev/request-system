-- +goose Up
-- +goose StatementBegin
CREATE TABLE orders (
    id SERIAL PRIMARY KEY,
    name VARCHAR(255) NOT NULL,

    department_id INT NOT NULL,
    otdel_id INT NOT NULL,           
    prorety_id INT NOT NULL,
    status_id INT NOT NULL,
    branch_id INT NOT NULL,
    office_id INT NOT NULL,
    equipment_id INT NOT NULL,
    user_id INT NOT NULL,

    duration INTERVAL NOT NULL,         
    address TEXT NOT NULL,

    created_at TIMESTAMP DEFAULT CURRENT_TIMESTAMP,
    updated_at TIMESTAMP DEFAULT CURRENT_TIMESTAMP,
    deleted_at TIMESTAMP DEFAULT NULL,

    CONSTRAINT fk_orders_department_id FOREIGN KEY (department_id) REFERENCES departments(id),
    CONSTRAINT fk_orders_otdel_id FOREIGN KEY (otdel_id) REFERENCES otdels(id),
    CONSTRAINT fk_orders_prorety_id FOREIGN KEY (prorety_id) REFERENCES proreties(id),
    CONSTRAINT fk_orders_status_id FOREIGN KEY (status_id) REFERENCES statuses(id),
    CONSTRAINT fk_orders_branch_id FOREIGN KEY (branch_id) REFERENCES branches(id),
    CONSTRAINT fk_orders_office_id FOREIGN KEY (office_id) REFERENCES offices(id),
    CONSTRAINT fk_orders_equipment_id FOREIGN KEY (equipment_id) REFERENCES equipments(id),
    CONSTRAINT fk_orders_user_id FOREIGN KEY (user_id) REFERENCES users(id)
);
-- +goose StatementEnd

-- +goose Down
-- +goose StatementBegin
DROP TABLE IF EXISTS orders;
-- +goose StatementEnd
