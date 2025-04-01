-- +goose Up
-- +goose StatementBegin
CREATE TABLE order_comments (
    id SERIAL PRIMARY KEY,
    message TEXT NOT NULL,
    status_id INT NOT NULL,
    order_id INT NOT NULL,
    user_id INT NOT NULL,

    created_at TIMESTAMP DEFAULT CURRENT_TIMESTAMP,
    updated_at TIMESTAMP DEFAULT CURRENT_TIMESTAMP,
    
    CONSTRAINT fk_order_comments_order_id FOREIGN KEY (order_id) REFERENCES orders(id),
    CONSTRAINT fk_order_comments_user_id FOREIGN KEY (user_id) REFERENCES users(id),
    CONSTRAINT fk_order_comments_status_id FOREIGN KEY (status_id) REFERENCES statuses(id)
);
-- +goose StatementEnd

-- +goose Down
-- +goose StatementBegin
DROP TABLE IF EXISTS order_comments;
-- +goose StatementEnd
