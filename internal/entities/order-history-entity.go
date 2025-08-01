package entities

import (
	"database/sql"
	"time"
)

type OrderHistory struct {
	ID           uint64        `db:"id"`
	OrderID      uint64        `db:"order_id"`
	UserID       uint64        `db:"user_id"`
	EventType    string        `db:"event_type"`
	OldValue     *string       `db:"old_value"`
	NewValue     *string       `db:"new_value"`
	Comment      *string       `db:"comment"`
	AttachmentID sql.NullInt64 `db:"attachment_id"`
	CreatedAt    time.Time     `db:"created_at"`
}
