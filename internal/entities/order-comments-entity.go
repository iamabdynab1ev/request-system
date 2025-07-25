package entities

import "request-system/pkg/types"

type OrderComment struct {
	ID       uint64 `json:"id"`
	Message  string `json:"message"`
	StatusID uint64 `json:"status_id"`
	OrderID  uint64 `json:"order_id"`
	UserID   uint64 `json:"user_id"`

	types.BaseEntity
}
