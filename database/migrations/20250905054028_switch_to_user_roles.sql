-- +goose Up
-- +goose StatementBegin
-- Шаг 1: Создаем новую таблицу user_roles для связи "многие-ко-многим".
-- Используем BIGINT для user_id и role_id для соответствия с типами SERIAL/BIGSERIAL.
CREATE TABLE user_roles (
    user_id BIGINT NOT NULL,
    role_id BIGINT NOT NULL,
    
    PRIMARY KEY (user_id, role_id),

    CONSTRAINT fk_user_roles_user_id
        FOREIGN KEY (user_id) REFERENCES users(id) ON DELETE CASCADE,
    
    CONSTRAINT fk_user_roles_role_id
        FOREIGN KEY (role_id) REFERENCES roles(id) ON DELETE CASCADE
);
COMMENT ON TABLE user_roles IS 'Связывает пользователей с их ролями (связь многие-ко-многим)';

-- Шаг 2: Копируем существующие данные из users.role_id в новую таблицу.
-- Это нужно для того, чтобы не потерять текущие роли пользователей.
INSERT INTO user_roles (user_id, role_id)
SELECT id, role_id FROM users WHERE role_id IS NOT NULL AND deleted_at IS NULL;

-- Шаг 3: Удаляем старый внешний ключ из таблицы users.
-- В PostgreSQL это действие может быть не нужно, если имя ключа генерируется автоматически,
-- но явно указать его - хорошая практика. Имя 'fk_roles_id' взято из вашей миграции.
ALTER TABLE users DROP CONSTRAINT IF EXISTS fk_roles_id;

-- Шаг 4: Удаляем старую колонку role_id из таблицы users.
ALTER TABLE users DROP COLUMN role_id;

-- +goose StatementEnd

-- +goose Down
-- +goose StatementBegin
-- Шаг 1: Возвращаем колонку role_id в таблицу users.
ALTER TABLE users ADD COLUMN role_id INT;
COMMENT ON COLUMN users.role_id IS 'ID роли пользователя (старая система)';

-- Шаг 2: Добавляем обратно внешний ключ.
ALTER TABLE users ADD CONSTRAINT fk_roles_id FOREIGN KEY (role_id) REFERENCES roles(id);

-- Шаг 3: Возвращаем данные обратно в users.role_id.
-- ВНИМАНИЕ: Если у пользователя было несколько ролей, здесь сохранится только ОДНА (первая найденная).
-- Это ограничение отката к старой системе.
UPDATE users u SET role_id = (
    SELECT ur.role_id FROM user_roles ur
    WHERE ur.user_id = u.id
    ORDER BY ur.role_id
    LIMIT 1
);

-- Шаг 4: Удаляем новую таблицу user_roles.
DROP TABLE user_roles;
-- +goose StatementEnd

