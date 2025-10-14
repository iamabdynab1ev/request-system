// Файл: internal/entities/order_type-entity.go

package entities

import "request-system/pkg/types"

type OrderType struct {
	ID       int     `json:"id"`
	Name     string  `json:"name"`
	Code     *string `json:"code"`
	StatusID int     `json:"status_id"`

	types.BaseEntity
}
