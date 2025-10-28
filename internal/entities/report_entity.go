package entities

import (
	"database/sql"
	"time"
)

type ReportFilter struct {
	DateFrom     *time.Time
	DateTo       *time.Time
	ExecutorIDs  []uint64
	OrderTypeIDs []uint64
	PriorityIDs  []uint64
	Page         int
	PerPage      int
}

// ReportItem определяет структуру одной строки в итоговом отчете.
type ReportItem struct {
	OrderID         uint64          `db:"order_id"`
	CreatorFio      sql.NullString  `db:"creator_fio"`
	CreatedAt       time.Time       `db:"created_at"`
	OrderTypeName   sql.NullString  `db:"order_type_name"`
	PriorityName    sql.NullString  `db:"priority_name"`
	StatusName      string          `db:"status_name"`
	OrderName       string          `db:"order_name"`
	ExecutorFio     sql.NullString  `db:"executor_fio"`
	DelegatedAt     sql.NullTime    `db:"delegated_at"`
	CompletedAt     sql.NullTime    `db:"completed_at"`
	ResolutionHours sql.NullFloat64 `db:"resolution_hours"`
	SLAStatus       string          `db:"sla_status"`
	Comment         sql.NullString  `db:"comment"`
}
