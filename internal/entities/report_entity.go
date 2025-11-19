package entities

import (
	"database/sql"
	"time"
)

type ReportFilter struct {
	DateFrom       *time.Time
	DateTo         *time.Time
	ExecutorIDs    []uint64
	OrderTypeIDs   []uint64
	PriorityIDs    []uint64
	Page           int
	PerPage        int
	Actor          *User
	PermissionsMap map[string]bool
}

type ReportItem struct {
	OrderID           int64          `db:"order_id"`
	CreatorFio        sql.NullString `db:"creator_fio"`
	CreatedAt         time.Time      `db:"created_at"`
	OrderTypeName     sql.NullString `db:"order_type_name"`
	PriorityName      sql.NullString `db:"priority_name"`
	StatusName        sql.NullString `db:"status_name"`
	OrderName         sql.NullString `db:"order_name"`
	ResponsibleFio    sql.NullString `db:"responsible_fio"`
	DelegatedAt       sql.NullTime   `db:"delegated_at"`
	ExecutorFio       sql.NullString `db:"executor_fio"`
	CompletedAt       sql.NullTime   `db:"completed_at"`
	ResolutionTimeStr sql.NullString `db:"resolution_time_str"`
	SLAStatus         sql.NullString `db:"sla_status"`
	SourceDepartment  sql.NullString `db:"source_department"`
	Comment           sql.NullString `db:"comment"`
}
