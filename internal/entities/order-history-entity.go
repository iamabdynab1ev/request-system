package entities

import (
	"database/sql"
	"time"

	"github.com/google/uuid"
)

type OrderHistory struct {
	ID           uint64         `db:"id"`
	OrderID      uint64         `db:"order_id"`
	UserID       uint64         `db:"user_id"`
	EventType    string         `db:"event_type"`
	OldValue     sql.NullString `db:"old_value"`
	NewValue     sql.NullString `db:"new_value"`
	Comment      sql.NullString `db:"comment"`
	CreatedAt    time.Time      `db:"created_at"`
	AttachmentID sql.NullInt64  `db:"attachment_id"`
	Metadata     []byte         `db:"metadata"`
	TxID         *uuid.UUID     `db:"tx_id"`
}
