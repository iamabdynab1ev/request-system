package entities

import "request-system/pkg/types"
import "time"

type Order struct {
	ID           int    `json:"id"`
	Name         string `json:"name"`

	DepartmentID int `json:"department_id"`
	OtdelID      int `json:"otdel_id"`
	ProretyID    int `json:"prorety_id"`
	StatusID     int `json:"status_id"`
	BranchID     int `json:"branch_id"`
	OfficeID     int `json:"office_id"`
	EquipmentID  int `json:"equipment_id"`
	UserID       int `json:"user_id"`

	Massage      string `json:"message"`
	ExecutorID   int    `json:"executor_id"`

	Duration     time.Duration `json:"duration"`
	Address      string        `json:"address"`

	types.BaseEntity
	types.SoftDelete
}