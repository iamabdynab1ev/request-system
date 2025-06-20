package dto

type CreateOrderDelegationDTO struct {
	DelegationUserID int `json:"delegation_user_id" validate:"required"`
	DelegatedUserID  int `json:"delegated_user_id" validate:"required"`
	StatusID         int `json:"status_id" validate:"required"`
	OrderID          int `json:"order_id" validate:"required"`
}

type UpdateOrderDelegationDTO struct {
	ID       int `json:"id" validate:"required"`
	StatusID int `json:"status_id" validate:"required, qt = 0"`
}

type OrderDelegationDTO struct {
	ID        uint64         `json:"id"`
	Delegator *ShortUserDTO   `json:"delegator"` 
	Delegatee *ShortUserDTO   `json:"delegatee"` 
	Status    ShortStatusDTO `json:"status"`
	Order     ShortOrderDTO  `json:"order"`
	CreatedAt string         `json:"created_at"`
	UpdatedAt string         `json:"updated_at"`
}

type ShortOrderDelegationDTO struct {
	ID               int `json:"id"`
	DelegationUserID int `json:"delegation_user_id"`
	DelegatedUserID  int `json:"delegated_user_id"`
	OrderID          int `json:"order_id"`
}
