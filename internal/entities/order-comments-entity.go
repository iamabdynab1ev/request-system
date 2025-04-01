package entities

import (
	"request-system/pkg/types"
)

type OrderComments struct {
	ID         int `json:"id"`
	Message     string `json:"message"`
	StatusID   int `json:"status_id"`
	OrderID int `json:"order_id"`
	UserID int `json:"user_id"`

	types.BaseEntity

}
