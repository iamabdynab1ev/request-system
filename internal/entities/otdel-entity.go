package entities

import "request-system/pkg/types"

type Otdel struct {
	ID           int    `json:"id"`
	Name         string `json:"name"`
	StatusID     int    `json:"status_id"`
	DepartmentsID int   `json:"departments_id"`

	types.BaseEntity
}