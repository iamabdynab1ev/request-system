-- file: testdata/schema.sql

-- Удаляем таблицы в правильном порядке, если они существуют, для полной очистки
DROP TABLE IF EXISTS order_comments CASCADE;
DROP TABLE IF EXISTS order_delegations CASCADE;
DROP TABLE IF EXISTS orders CASCADE;
DROP TABLE IF EXISTS users CASCADE;
DROP TABLE IF EXISTS statuses CASCADE;
DROP TABLE IF EXISTS proreties CASCADE;


-- Создаем таблицы заново
CREATE TABLE users (
    id SERIAL PRIMARY KEY,
    fio VARCHAR(255) NOT NULL
    -- Добавьте остальные поля, если они есть у вас в реальной таблице
);

CREATE TABLE statuses (
    id SERIAL PRIMARY KEY,
    name VARCHAR(255) NOT NULL
);

CREATE TABLE proreties (
    id SERIAL PRIMARY KEY,
    name VARCHAR(255) NOT NULL
);

CREATE TABLE orders (
    id BIGSERIAL PRIMARY KEY,
    name VARCHAR(255),
    department_id INT,
    otdel_id INT,
    branch_id INT,
    office_id INT,
    equipment_id INT,
    duration VARCHAR(255),
    address VARCHAR(255),
    user_id INT REFERENCES users(id),
    executor_id INT REFERENCES users(id),
    status_id INT REFERENCES statuses(id),
    prorety_id INT REFERENCES proreties(id),
    created_at TIMESTAMP WITH TIME ZONE,
    updated_at TIMESTAMP WITH TIME ZONE,
    deleted_at TIMESTAMP WITH TIME ZONE
);

CREATE TABLE order_delegations (
    id SERIAL PRIMARY KEY,
    order_id BIGINT REFERENCES orders(id) ON DELETE CASCADE,
    delegation_user_id INT REFERENCES users(id),
    delegated_user_id INT REFERENCES users(id),
    status_id INT REFERENCES statuses(id),
    created_at TIMESTAMP WITH TIME ZONE,
    updated_at TIMESTAMP WITH TIME ZONE,
     deleted_at TIMESTAMP WITH TIME ZONE
);

CREATE TABLE order_comments (
    id SERIAL PRIMARY KEY,
    order_id BIGINT REFERENCES orders(id) ON DELETE CASCADE,
    user_id INT REFERENCES users(id),
    message TEXT,
    status_id INT REFERENCES statuses(id),
    created_at TIMESTAMP WITH TIME ZONE,
    updated_at TIMESTAMP WITH TIME ZONE,
     deleted_at TIMESTAMP WITH TIME ZONE
);