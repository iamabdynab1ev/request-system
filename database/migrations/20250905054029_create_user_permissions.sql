-- +goose Up
-- +goose StatementBegin
-- Создаем таблицу для хранения индивидуальных разрешений, выданных пользователю напрямую.
CREATE TABLE user_permissions (
    user_id BIGINT NOT NULL,
    permission_id BIGINT NOT NULL,
    
    PRIMARY KEY (user_id, permission_id),

    CONSTRAINT fk_user_permissions_user_id
        FOREIGN KEY (user_id) REFERENCES users(id) ON DELETE CASCADE,
    
    CONSTRAINT fk_user_permissions_permission_id
        FOREIGN KEY (permission_id) REFERENCES permissions(id) ON DELETE CASCADE
);
COMMENT ON TABLE user_permissions IS 'Индивидуальные РАЗРЕШЕНИЯ для пользователей, в обход ролей';
-- +goose StatementEnd

-- +goose Down
-- +goose StatementBegin
DROP TABLE IF EXISTS user_permissions;
-- +goose StatementEnd
