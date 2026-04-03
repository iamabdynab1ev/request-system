-- +goose Up
-- +goose StatementBegin
-- Создаем таблицу для хранения явных ЗАПРЕТОВ на определенные права для пользователя.
-- Эта таблица имеет наивысший приоритет при проверке доступа.
CREATE TABLE user_permission_denials (
    user_id BIGINT NOT NULL,
    permission_id BIGINT NOT NULL,
    
    PRIMARY KEY (user_id, permission_id),

    CONSTRAINT fk_user_permission_denials_user_id
        FOREIGN KEY (user_id) REFERENCES users(id) ON DELETE CASCADE,
    
    CONSTRAINT fk_user_permission_denials_permission_id
        FOREIGN KEY (permission_id) REFERENCES permissions(id) ON DELETE CASCADE
);
COMMENT ON TABLE user_permission_denials IS 'Индивидуальные ЗАПРЕТЫ для пользователей, перекрывающие права от ролей';
-- +goose StatementEnd

-- +goose Down
-- +goose StatementBegin
DROP TABLE IF EXISTS user_permission_denials;
-- +goose StatementEnd

