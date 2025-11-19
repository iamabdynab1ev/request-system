package entities

import "time"

type Position struct {
	ID           uint64    `db:"id"`
	Name         string    `db:"name"`
	StatusID     *uint64   `db:"status_id"`
	DepartmentID *uint64   `db:"department_id"`
	OtdelID      *uint64   `db:"otdel_id"`
	BranchID     *uint64   `db:"branch_id"`
	OfficeID     *uint64   `db:"office_id"`
	Type         *string   `db:"type"`
	CreatedAt    time.Time `db:"created_at"`
	UpdatedAt    time.Time `db:"updated_at"`
	ExternalID   *string   `db:"external_id"`
	SourceSystem *string   `db:"source_system"`
}
