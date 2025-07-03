-- +goose Up
-- +goose StatementBegin
CREATE TABLE users (
    id SERIAL PRIMARY KEY,

    fio VARCHAR(100) NOT NULL,
    email VARCHAR(50) UNIQUE NOT NULL,
    phone_number VARCHAR(12) UNIQUE NOT NULL,
    password VARCHAR(60) NOT NULL,
    
    status_id INT NOT NULL,
    role_id INT NOT NULL,
    department_id INT NOT NULL,

    position VARCHAR(255), 
    branch_id INT,
    office_id INT,
    otdel_id INT,

    created_at TIMESTAMP WITH TIME ZONE DEFAULT CURRENT_TIMESTAMP,
    updated_at TIMESTAMP WITH TIME ZONE DEFAULT CURRENT_TIMESTAMP,
    deleted_at TIMESTAMP WITH TIME ZONE,

    CONSTRAINT fk_status_id FOREIGN KEY (status_id) REFERENCES statuses(id),
    CONSTRAINT fk_roles_id FOREIGN KEY (role_id) REFERENCES roles(id),
    CONSTRAINT fk_departments_id FOREIGN KEY (department_id) REFERENCES departments(id),
    
    CONSTRAINT fk_branches_id FOREIGN KEY (branch_id) REFERENCES branches(id) ON DELETE SET NULL,
    CONSTRAINT fk_offices_id FOREIGN KEY (office_id) REFERENCES offices(id) ON DELETE SET NULL,
    CONSTRAINT fk_otdels_id FOREIGN KEY (otdel_id) REFERENCES otdels(id) ON DELETE SET NULL
);
-- +goose StatementEnd

-- +goose Down
-- +goose StatementBegin
DROP TABLE IF EXISTS users;
-- +goose StatementEnd