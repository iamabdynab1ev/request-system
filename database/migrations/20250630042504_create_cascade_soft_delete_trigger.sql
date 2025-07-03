

-- +goose Up
-- +goose StatementBegin
CREATE OR REPLACE FUNCTION cascade_soft_delete_for_orders()
RETURNS TRIGGER AS $$
BEGIN
    IF (TG_OP = 'UPDATE') THEN
        IF OLD.deleted_at IS NULL AND NEW.deleted_at IS NOT NULL THEN
            UPDATE order_comments SET deleted_at = NEW.deleted_at WHERE order_id = NEW.id;
            UPDATE order_delegations SET deleted_at = NEW.deleted_at WHERE order_id = NEW.id;
        ELSIF OLD.deleted_at IS NOT NULL AND NEW.deleted_at IS NULL THEN
            UPDATE order_comments SET deleted_at = NULL WHERE order_id = NEW.id;
            UPDATE order_delegations SET deleted_at = NULL WHERE order_id = NEW.id;
        END IF;
    END IF;
    RETURN NEW;
END;
$$ LANGUAGE plpgsql;
-- +goose StatementEnd

-- +goose StatementBegin
CREATE TRIGGER trg_cascade_soft_delete
AFTER UPDATE ON orders
FOR EACH ROW
EXECUTE FUNCTION cascade_soft_delete_for_orders();
-- +goose StatementEnd


-- +goose Down
DROP TRIGGER IF EXISTS trg_cascade_soft_delete ON orders;
DROP FUNCTION IF EXISTS cascade_soft_delete_for_orders();