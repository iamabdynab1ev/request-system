-- +goose Up
-- +goose StatementBegin
CREATE TABLE role_permission (
 
    id SERIAL PRIMARY KEY,
    role_id INTEGER NOT NULL,
    permission_id INTEGER NOT NULL,
    
 
    CONSTRAINT fk_role_permission_role_id FOREIGN KEY (role_id) REFERENCES roles(id) ON DELETE CASCADE,
    CONSTRAINT fk_role_permission_permission_id FOREIGN KEY (permission_id) REFERENCES permissions(id) ON DELETE CASCADE,

    CONSTRAINT role_permission_unique UNIQUE (role_id, permission_id)
);
-- +goose StatementEnd

-- +goose Down
-- +goose StatementBegin
DROP TABLE IF EXISTS role_permission;
-- +goose StatementEnd