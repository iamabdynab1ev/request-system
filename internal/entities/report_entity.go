package entities

import (
	"database/sql"
	"encoding/json"
	"time"
)

type ReportFilter struct {
	OrderIDs     []uint64
	UserIDs      []uint64
	EventTypes   []string
	DateFrom     *time.Time
	DateTo       *time.Time
	MetadataJSON string
	Page         int
	PerPage      int
	SortOrder    string
}

type HistoryReportItem struct {
	ID        uint64          `json:"id" db:"id"`
	OrderID   uint64          `json:"order_id" db:"order_id"`
	OrderName string          `json:"order_name" db:"order_name"`
	UserID    uint64          `json:"user_id" db:"user_id"`
	UserName  string          `json:"user_name" db:"user_name"`
	EventType string          `json:"event_type" db:"event_type"`
	OldValue  sql.NullString  `json:"old_value" db:"old_value"`
	NewValue  sql.NullString  `json:"new_value" db:"new_value"`
	Comment   sql.NullString  `json:"comment" db:"comment"`
	Metadata  json.RawMessage `json:"metadata" db:"metadata"`
	CreatedAt time.Time       `json:"created_at" db:"created_at"`
}
