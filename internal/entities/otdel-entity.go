package entities

import (
	"request-system/pkg/types"
)
type Otdel struct {
	ID           int       `json:"id"`
	Name         string    `json:"name"`
	Status       Status    `json:"status"`
	DepartmentID int       `json:"department_id"`
	
	types.BaseEntity
}
