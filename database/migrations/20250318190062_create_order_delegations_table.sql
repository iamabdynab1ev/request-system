-- +goose Up
-- +goose StatementBegin
CREATE TABLE order_delegations (
    id SERIAL PRIMARY KEY,
    delegation_user_id INT NOT NULL,
    delegated_user_id INT NOT NULL,
    status_id INT NOT NULL,
    order_id INT NOT NULL,
   
    created_at TIMESTAMP WITH TIME ZONE DEFAULT CURRENT_TIMESTAMP,
    updated_at TIMESTAMP WITH TIME ZONE DEFAULT CURRENT_TIMESTAMP,

    CONSTRAINT fk_order_delegations_delegation_user_id FOREIGN KEY (delegation_user_id) REFERENCES users(id),
    CONSTRAINT fk_order_delegations_delegated_user_id FOREIGN KEY (delegated_user_id) REFERENCES users(id),
    CONSTRAINT fk_order_delegations_status_id FOREIGN KEY (status_id) REFERENCES statuses(id),
    CONSTRAINT fk_order_delegations_order_id FOREIGN KEY (order_id) REFERENCES orders(id)
);
-- +goose StatementEnd

-- +goose Down
-- +goose StatementBegin
DROP TABLE IF EXISTS order_delegations;
-- +goose StatementEnd
