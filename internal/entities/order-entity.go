package entities

import (
	"time"
	"request-system/pkg/types"

)

type Order struct {
	ID           int        `json:"id"`
	Name         string     `json:"name"`
	DepartmentID int        `json:"department_id"`
	OrderID      int        `json:"order_id"`        
	ProretyID    int        `json:"prorety_id"`      
	StatusID     int        `json:"status_id"`
	BranchID     int        `json:"branch_id"`
	OfficeID     int        `json:"office_id"`
	EquipmentID  int        `json:"equipment_id"`
	UserID       int        `json:"user_id"`
	Duration     time.Time  `json:"duration"`
	Address      string     `json:"address"`
	
	types.BaseEntity 
	types.SoftDelete
}