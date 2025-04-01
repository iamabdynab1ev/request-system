package entities

import (
	"request-system/pkg/types"
)
	type Equipment struct {
	ID          int        `json:"id"`
	Name        string     `json:"name"`
	Address     string     `json:"address"`
	BranchID    int        `json:"branch_id"`
	TypeID 	 	int 	   `json:"type_id"`
	OfficeID    int        `json:"office_id"`
	StatusID    int        `json:"status_id"`

	types.BaseEntity
}

