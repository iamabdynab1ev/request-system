package dto

type CreateOrderCommentDTO struct {
	Comment string `json:"comment" validarte:"required,max=255"`
}

type UpdateOrderCommentDTO struct {
	ID      int    `json:"id" validate:"required"`
	OrderID int    `json:"order_id" validate:"required"`
	Comment string `json:"comment" validate:"required,max=255"`
}

type OrderCommentDTO struct {
	ID        int    `json:"id"`
	OrderID   int    `json:"order_id"`
	Comment   string `json:"comment"`
	CreatedAt string `json:"created_at"`
}

type ShortOrderCommentDTO struct {
	ID      int    `json:"id"`
	OrderID int    `json:"order_id"`
	Comment string `json:"comment"`
}


