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
	PositionType string `json:"position_type" validate:"required"`
	StatusID     int    `json:"status_id" validate:"required"`
}

type UpdateOrderRoutingRuleDTO struct {
	RuleName     null.String `json:"name,omitempty"`
	OrderTypeID  null.Int    `json:"order_type_id"`
	DepartmentID null.Int    `json:"department_id,omitempty"` // <-- 1. ДОБАВЬТЕ ПРЕФИКС utils.NullableInt `json:"department_id,omitempty"` // <-- 2. ДОБАВЬТЕ ПРЕФИКС utils.
	OtdelID      null.Int    `json:"otdel_id,omitempty"`      // <-- 2. ДОБАВЬТЕ ПРЕФИКС utils.
	PositionType null.String `json:"position_type,omitempty"`
	StatusID     null.Int    `json:"status_id,omitempty"`
}

type OrderRoutingRuleResponseDTO struct {
	ID               uint64   `json:"id"`
	RuleName         string   `json:"name"`
	OrderTypeID      *int     `json:"order_type_id"`
	DepartmentID     *int     `json:"department_id"`
	OtdelID          *int     `json:"otdel_id"`
	PositionID       *int     `json:"position_id,omitempty"`        // Старый ID, который хранится в базе
	PositionType     string   `json:"position_type,omitempty"`      // Системное имя типа
	PositionTypeName string   `json:"position_type_name,omitempty"` // Человекочитаемое имя типа
	RequiredFields   []string `json:"required_fields,omitempty"`
	StatusID         int      `json:"status_id"`
	CreatedAt        string   `json:"created_at"`
	UpdatedAt        string   `json:"updated_at,omitempty"`
}
