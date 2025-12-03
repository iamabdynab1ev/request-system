package entities

import "request-system/pkg/types"

type Otdel struct {
	ID            uint64  `json:"id"`
	Name          string  `json:"name"`
	StatusID      uint64  `json:"status_id"`
	DepartmentsID *uint64 `json:"department_id" db:"departments_id"`
	BranchID      *uint64 `json:"branch_id" db:"branch_id"`
	ParentID      *uint64 `json:"parent_id" db:"parent_id"`
	ExternalID    *string `db:"external_id"`
	SourceSystem  *string `db:"source_system"`
	Parent        *Otdel
	types.BaseEntity
}
