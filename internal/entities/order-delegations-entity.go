package entities

import (
	"request-system/pkg/types"
)

type OrderDelegation struct {
	ID               int    `json:"id"`
	DelegationUserID int    `json:"delegation_user_id"`
	DeletedUserID    int    `json:"deleted_user_id"`
	OrderID          int    `json:"order_id"`
	StatusID         int    `json:"status_id"`
	CreatedAt        string `json:"created_at"`
	UpdatedAt        string `json:"updated_at"`

	types.BaseEntity
}
