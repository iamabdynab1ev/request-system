-- +goose Up
-- +goose StatementBegin
CREATE TABLE attachments (
    id BIGSERIAL PRIMARY KEY,                    
    order_id BIGINT NOT NULL,                   
    user_id BIGINT NOT NULL,                     
    file_name VARCHAR(255) NOT NULL,             
    file_path VARCHAR(255) NOT NULL,            
    file_type VARCHAR(50) NOT NULL,             
    file_size BIGINT NOT NULL,                   
    created_at TIMESTAMPTZ DEFAULT NOW(),

    CONSTRAINT fk_attachments_order FOREIGN KEY (order_id) REFERENCES orders(id) ON DELETE CASCADE,
    CONSTRAINT fk_attachments_user FOREIGN KEY (user_id) REFERENCES users(id) ON DELETE RESTRICT
);

CREATE INDEX idx_attachments_order_id ON attachments(order_id);
-- +goose StatementEnd

-- +goose Down
-- +goose StatementBegin
DROP INDEX IF EXISTS idx_attachments_order_id;
DROP TABLE IF EXISTS attachments;
-- +goose StatementEnd
