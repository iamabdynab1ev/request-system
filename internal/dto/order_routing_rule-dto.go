// Файл: internal/dto/order_routing_rule-dto.go
package dto

type CreateOrderRoutingRuleDTO struct {
	RuleName     string `json:"name" validate:"required"`
	OrderTypeID  *int   `json:"order_type_id"`
	DepartmentID *int   `json:"department_id"`
	OtdelID      *int   `json:"otdel_id"`
	PositionID   int    `json:"position_id" validate:"required"`
	StatusID     int    `json:"status_id" validate:"required"`
}

type UpdateOrderRoutingRuleDTO struct {
	RuleName     *string `json:"name,omitempty"`
	OrderTypeID  *int    `json:"order_type_id"`
	DepartmentID *int    `json:"department_id"`
	OtdelID      *int    `json:"otdel_id"`
	PositionID   *int    `json:"position_id"`
	StatusID     *int    `json:"status_id,omitempty"`
}

type OrderRoutingRuleResponseDTO struct {
	ID             uint64   `json:"id"`
	RuleName       string   `json:"name"`
	OrderTypeID    *int     `json:"order_type_id"`
	DepartmentID   *int     `json:"department_id"`
	OtdelID        *int     `json:"otdel_id"`
	PositionID     *int     `json:"position_id"`
	RequiredFields []string `json:"required_fields"`
	StatusID       int      `json:"status_id"`
	CreatedAt      string   `json:"created_at"`
	UpdatedAt      string   `json:"updated_at,omitempty"`
}
