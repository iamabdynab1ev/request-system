package entities

import "time"

// Order — структура для заявки
type Order struct {
	ID                       uint64     `db:"id"`
	Name                     string     `db:"name"`
	DepartmentID             *uint64    `db:"department_id"`
	StatusID                 uint64     `db:"status_id"`
	CreatorID                uint64     `db:"user_id"`
	OrderTypeID              *uint64    `db:"order_type_id"`
	OtdelID                  *uint64    `db:"otdel_id"`
	PriorityID               *uint64    `db:"priority_id"`
	BranchID                 *uint64    `db:"branch_id"`
	OfficeID                 *uint64    `db:"office_id"`
	EquipmentID              *uint64    `db:"equipment_id"`
	EquipmentTypeID          *uint64    `db:"equipment_type_id"`
	ExecutorID               *uint64    `db:"executor_id"`
	Duration                 *time.Time `db:"duration"`
	Address                  *string    `db:"address"`
	CreatedAt                time.Time  `db:"created_at"`
	UpdatedAt                time.Time  `db:"updated_at"`
	DeletedAt                *time.Time `db:"deleted_at"`
	CompletedAt              *time.Time `db:"completed_at"`
	FirstResponseTimeSeconds *uint64    `db:"first_response_time_seconds"`
	ResolutionTimeSeconds    *uint64    `db:"resolution_time_seconds"`
	IsFirstContactResolution *bool      `db:"is_first_contact_resolution"`
}
