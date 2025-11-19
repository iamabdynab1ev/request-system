package entities

import "request-system/pkg/types"

type Otdel struct {
	ID            uint64  `json:"id"`
	Name          string  `json:"name"`
	StatusID      uint64  `json:"status_id"`
	DepartmentsID uint64  `json:"department_id"`
	ExternalID    *string `db:"external_id"`
	SourceSystem  *string `db:"source_system"`
	types.BaseEntity
}
