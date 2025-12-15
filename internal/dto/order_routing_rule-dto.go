// Файл: internal/dto/order_routing_rule-dto.go
package dto

import (
	"github.com/aarondl/null/v8"
)

type CreateOrderRoutingRuleDTO struct {
	RuleName     string `json:"name" validate:"required"`
	OrderTypeID  *int   `json:"order_type_id"`
	DepartmentID *int   `json:"department_id"`
	OtdelID      *int   `json:"otdel_id"`
	BranchID     *int   `json:"branch_id"`
	OfficeID     *int   `json:"office_id"`
	PositionType string `json:"position_type" validate:"required"`
	StatusID     int    `json:"status_id" validate:"required"`
}

type UpdateOrderRoutingRuleDTO struct {
	RuleName     null.String `json:"name,omitempty"`
	OrderTypeID  null.Int    `json:"order_type_id"`
	DepartmentID null.Int    `json:"department_id,omitempty"`
	OtdelID      null.Int    `json:"otdel_id,omitempty"`
	BranchID     null.Int    `json:"branch_id,omitempty"`
	OfficeID     null.Int    `json:"office_id,omitempty"`
	PositionType null.String `json:"position_type,omitempty"`
	StatusID     null.Int    `json:"status_id,omitempty"`
}

type OrderRoutingRuleResponseDTO struct {
	ID               uint64   `json:"id"`
	RuleName         string   `json:"name"`
	OrderTypeID      *int     `json:"order_type_id"`
	DepartmentID     *int     `json:"department_id"`
	OtdelID          *int     `json:"otdel_id"`
	BranchID         *int     `json:"branch_id"`
	OfficeID         *int     `json:"office_id"`
	PositionID       *int     `json:"position_id,omitempty"`
	PositionType     string   `json:"position_type,omitempty"`
	PositionTypeName string   `json:"position_type_name,omitempty"`
	RequiredFields   []string `json:"required_fields,omitempty"`
	StatusID         int      `json:"status_id"`
	CreatedAt        string   `json:"created_at"`
	UpdatedAt        string   `json:"updated_at,omitempty"`
}
