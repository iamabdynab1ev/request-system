package entities

import (
	"database/sql"
	"time"
)

type Order struct {
	ID              uint64         `db:"id"`
	Name            string         `db:"name"`
	DepartmentID    uint64         `db:"department_id"`
	OrderTypeID     uint64         `db:"order_type_id"`
	OtdelID         *uint64        `db:"otdel_id"`
	PriorityID      *uint64        `db:"priority_id"`
	StatusID        uint64         `db:"status_id"`
	BranchID        *uint64        `db:"branch_id"`
	OfficeID        *uint64        `db:"office_id"`
	EquipmentID     *uint64        `db:"equipment_id"`
	EquipmentTypeID *uint64        `db:"equipment_type_id"`
	CreatorID       uint64         `db:"user_id"`
	ExecutorID      *uint64        `db:"executor_id"`
	Duration        *time.Time     `db:"duration"`
	Address         *string        `db:"address"`
	StatusName      sql.NullString `db:"status_name"`
	PriorityName    sql.NullString `db:"priority_name"`
	CreatorFIO      sql.NullString `db:"creator_fio"`
	ExecutorFIO     sql.NullString `db:"executor_fio"`
	CreatedAt       time.Time      `db:"created_at"`
	UpdatedAt       time.Time      `db:"updated_at"`
	DeletedAt       sql.NullTime   `db:"deleted_at"`
}
