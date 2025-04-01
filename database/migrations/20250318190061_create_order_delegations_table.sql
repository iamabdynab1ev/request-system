-- +goose Up
-- +goose StatementBegin
CREATE TABLE order_delegations (
    id SERIAL PRIMARY KEY,
    delegation_user_id INT NOT NULL,
    delegated_user_id INT NOT NULL,
    created_at TIMESTAMP DEFAULT CURRENT_TIMESTAMP,
    updated_at TIMESTAMP DEFAULT CURRENT_TIMESTAMP,
    
    CONSTRAINT fk_order_delegations_delegation_user_id FOREIGN KEY (delegation_user_id) REFERENCES users(id),
    CONSTRAINT fk_order_delegations_delegated_user_id FOREIGN KEY (delegated_user_id) REFERENCES users(id)
);
-- +goose StatementEnd

-- +goose Down
-- +goose StatementBegin
DROP TABLE IF EXISTS order_delegations;
-- +goose StatementEnd
