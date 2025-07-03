-- +goose Up

ALTER TABLE orders ADD COLUMN deleted_at TIMESTAMP WITH TIME ZONE;
ALTER TABLE order_comments ADD COLUMN deleted_at TIMESTAMP WITH TIME ZONE;
ALTER TABLE order_delegations ADD COLUMN deleted_at TIMESTAMP WITH TIME ZONE;


CREATE INDEX idx_orders_deleted_at ON orders(deleted_at);
CREATE INDEX idx_order_comments_deleted_at ON order_comments(deleted_at);
CREATE INDEX idx_order_delegations_deleted_at ON order_delegations(deleted_at);


-- +goose Down

DROP INDEX IF EXISTS idx_orders_deleted_at;
DROP INDEX IF EXISTS idx_order_comments_deleted_at;
DROP INDEX IF EXISTS idx_order_delegations_deleted_at;

ALTER TABLE orders DROP COLUMN IF EXISTS deleted_at;
ALTER TABLE order_comments DROP COLUMN IF EXISTS deleted_at;
ALTER TABLE order_delegations DROP COLUMN IF EXISTS deleted_at;