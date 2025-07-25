package entities

import "time"

type Order struct {
	ID           uint64     `db:"id"`
	Name         string     `db:"name"`
	DepartmentID uint64     `db:"department_id"`
	OtdelID      *uint64    `db:"otdel_id"`
	PriorityID   uint64     `db:"priority_id"`
	StatusID     uint64     `db:"status_id"`
	BranchID     *uint64    `db:"branch_id"`
	OfficeID     *uint64    `db:"office_id"`
	EquipmentID  *uint64    `db:"equipment_id"`
	CreatorID    uint64     `db:"user_id"`
	ExecutorID   uint64     `db:"executor_id"`
	Duration     *string    `db:"duration"`
	Address      string     `db:"address"`
	CreatedAt    time.Time  `db:"created_at"`
	UpdatedAt    time.Time  `db:"updated_at"`
	DeletedAt    *time.Time `db:"deleted_at"`
}
