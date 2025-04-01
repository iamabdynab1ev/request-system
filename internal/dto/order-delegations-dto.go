package dto

type CreateOrderDelegationDTO struct {
	DelegationUserID int   `json:"delegation_user_id" validate:"required"`
	DeletedUserID    int   `json:"deleted_user_id" validate:"required"`
}

type UpdateOrderDelegationDTO struct {
	ID               int   `json:"id" validate:"required"`
	DelegationUserID int   `json:"delegation_user_id"  validate:"required"`
	DelegatedUserID  int   `json:"deleted_user_id" validate:"required"`
}

type OrderDelegationDTO struct {
	ID               int    `json:"id"`
	DelegationUserID int    `json:"delegation_user_id"`
	DelegatedUserID  int    `json:"deleted_user_id"`
	CreatedAt        string `json:"created_at"`
}

type ShortOrderDelegationDTO struct {
	ID               int    `json:"id"`
	DelegationUserID int    `json:"delegation_user_id"`
	DeletedUserID    int    `json:"deleted_user_id"`
}

