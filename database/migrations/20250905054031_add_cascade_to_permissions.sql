-- +goose Up
-- +goose StatementBegin
-- Обновляем user_permissions
ALTER TABLE user_permissions DROP CONSTRAINT fk_user_permissions_permission_id;
ALTER TABLE user_permissions ADD CONSTRAINT fk_user_permissions_permission_id 
    FOREIGN KEY (permission_id) REFERENCES permissions(id) ON DELETE CASCADE;

-- Обновляем user_permission_denials
ALTER TABLE user_permission_denials DROP CONSTRAINT fk_user_permission_denials_permission_id;
ALTER TABLE user_permission_denials ADD CONSTRAINT fk_user_permission_denials_permission_id 
    FOREIGN KEY (permission_id) REFERENCES permissions(id) ON DELETE CASCADE;

-- Обновляем role_permissions
ALTER TABLE role_permissions DROP CONSTRAINT fk_role_permissions_permission_id;
ALTER TABLE role_permissions ADD CONSTRAINT fk_role_permissions_permission_id 
    FOREIGN KEY (permission_id) REFERENCES permissions(id) ON DELETE CASCADE;
-- +goose StatementEnd

-- +goose Down
-- +goose StatementBegin
-- Откат (возвращаем старое поведение)
ALTER TABLE user_permissions DROP CONSTRAINT fk_user_permissions_permission_id;
ALTER TABLE user_permissions ADD CONSTRAINT fk_user_permissions_permission_id 
    FOREIGN KEY (permission_id) REFERENCES permissions(id);

ALTER TABLE user_permission_denials DROP CONSTRAINT fk_user_permission_denials_permission_id;
ALTER TABLE user_permission_denials ADD CONSTRAINT fk_user_permission_denials_permission_id 
    FOREIGN KEY (permission_id) REFERENCES permissions(id);

ALTER TABLE role_permissions DROP CONSTRAINT fk_role_permissions_permission_id;
ALTER TABLE role_permissions ADD CONSTRAINT fk_role_permissions_permission_id 
    FOREIGN KEY (permission_id) REFERENCES permissions(id);
-- +goose StatementEnd