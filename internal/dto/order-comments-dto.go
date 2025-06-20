package dto

type CreateOrderCommentDTO struct {
	Message  string `json:"message" validate:"required"`
	StatusID int    `json:"status_id" validate:"required"`
	OrderID  int    `json:"order_id" validate:"required"`
	UserID   int    `json:"user_id" validate:"required"` 
}

type UpdateOrderCommentDTO struct {
	ID       int    `json:"id" validate:"required"`
	Message  string `json:"message" validate:"omitempty"`
	StatusID int    `json:"status_id" validate:"omitempty"`
	OrderID  int    `json:"order_id" validate:"omitempty"`
	UserID   int    `json:"user_id" validate:"omitempty"`
}

type OrderCommentDTO struct {
	ID        int          `json:"id"`
	Message   string       `json:"message"`
	Status    ShortStatusDTO  `json:"status"`  
	Order     ShortOrderDTO `json:"order"`  

	Author    ShortUserDTO `json:"author"` 

	CreatedAt string `json:"created_at"`
	UpdatedAt string `json:"updated_at"`
}

type ShortOrderCommentDTO struct {
	ID      int    `json:"id"`
	Message string `json:"message"`
	OrderID int    `json:"order_id"`
}