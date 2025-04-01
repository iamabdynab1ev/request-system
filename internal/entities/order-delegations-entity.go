package entities


import (
    "request-system/pkg/types"
)

type OrderDelegation struct {
	ID                int       `json:"id"`
	DelegationUserID  int       `json:"delegation_user_id"`
	DeletedUserID     int       `json:"deleted_user_id"`

	types.BaseEntity 
}




