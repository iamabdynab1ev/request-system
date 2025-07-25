package entities

import (
	"request-system/pkg/types"
)

type OrderDelegation struct {
	ID               uint64    `json:"id"`
	DelegationUserID uint64    `json:"delegation_user_id"`
	DeletedUserID    uint64    `json:"deleted_user_id"`
	OrderID          uint64    `json:"order_id"`
	StatusID         uint64    `json:"status_id"`
	CreatedAt        string `json:"created_at"`
	UpdatedAt        string `json:"updated_at"`

	types.BaseEntity
}
