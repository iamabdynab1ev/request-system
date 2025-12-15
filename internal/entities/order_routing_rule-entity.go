package entities

import "request-system/pkg/types"

type OrderRoutingRule struct {
	ID           int    `json:"id" db:"id"`
	RuleName     string `json:"name" db:"rule_name"`
	OrderTypeID  *int   `json:"order_type_id" db:"order_type_id"`
	DepartmentID *int   `json:"department_id" db:"department_id"`
	OtdelID      *int   `json:"otdel_id" db:"otdel_id"`
	BranchID     *int   `json:"branch_id" db:"branch_id"`
	OfficeID     *int   `json:"office_id" db:"office_id"`
	PositionID   *int   `json:"position_id" db:"assign_to_position_id"`
	StatusID     int    `json:"status_id" db:"status_id"`

	types.BaseEntity
}
