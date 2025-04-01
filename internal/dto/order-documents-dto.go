package dto
	
type CreateOrderDocumentDTO struct {
	Name    string `json:"name" validate:"required,max=255"`
	Path    string `json:"path" validate:"required,max=255"`
	Type    string `json:"type" validate:"required,max=50"`
	OrderID int    `json:"order_id" validate:"required"`
}

type UpdateOrderDocumentDTO struct {
	ID      int    `json:"id" validate:"required"`
	Name    string `json:"name" validate:"omitempty,max=50"`
	Path    string `json:"path" validate:"omotempty,max=200"`
	Type    string `json:"type" validate:"omotempty,max=50"`
	OrderID int    `json:"order_id" validate:"required"`
}

type OrderDocumentDTO struct {
	ID        int    `json:"id"`
	Name      string `json:"name"`
	Path      string `json:"path"`
	Type      string `json:"type"`
	OrderID   int    `json:"order_id"`
	CreatedAt string `json:"created_at"`
}

type ShortOrderDocumentDTO struct {
	ID      int    `json:"id"`
	Name    string `json:"name"`
	Path    string `json:"path"`
	Type    string `json:"type"`
}


