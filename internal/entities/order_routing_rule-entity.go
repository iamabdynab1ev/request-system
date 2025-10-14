// Файл: internal/entities/order_routing_rule-entity.go
package entities

import "request-system/pkg/types"

type OrderRoutingRule struct {
	ID           int    `json:"id"`
	RuleName     string `json:"name"`
	OrderTypeID  *int   `json:"order_type_id"`
	DepartmentID *int   `json:"department_id"`
	OtdelID      *int   `json:"otdel_id"`
	PositionID   *int   `json:"position_id"`
	StatusID     int    `json:"status_id"`
	types.BaseEntity
}
