-- +goose Up
-- +goose StatementBegin
CREATE TABLE equipments (
    id SERIAL PRIMARY KEY,
    name VARCHAR(255) NOT NULL,
    address VARCHAR(255) NOT NULL,
   
    branch_id INT  NOT NULL,
    office_id INT NOT NULL,
    status_id INT NOT NULL,
    equipment_type_id INT NOT NULL,
    
    created_at TIMESTAMP DEFAULT CURRENT_TIMESTAMP,
    updated_at TIMESTAMP DEFAULT CURRENT_TIMESTAMP,

    CONSTRAINT fk_equipment_branch_id FOREIGN KEY (branch_id) REFERENCES branches(id),
    CONSTRAINT fk_equipment_equipment_type_id FOREIGN KEY (equipment_type_id) REFERENCES equipment_types(id),
    CONSTRAINT fk_equipment_office_id FOREIGN KEY (office_id) REFERENCES offices(id),
    CONSTRAINT fk_equipment_status_id FOREIGN KEY (status_id) REFERENCES statuses(id)

);

-- +goose StatementEnd

-- +goose Down
-- +goose StatementBegin
DROP TABLE IF EXISTS equipments;
-- +goose StatementEnd

