package dto

type CreateOrderDocumentDTO struct {
	Name string `json:"name" validate:"required"`
	Path string `json:"path" validate:"required"`
	Type string `json:"type" validate:"required"`

	OrderID int `json:"order_id" validate:"required"`
}

type UpdateOrderDocumentDTO struct {
	ID   int    `json:"id" validate:"required"`
	Name string `json:"name" validate:"omitempty"`
	Path string `json:"path" validate:"omitempty"`
	Type string `json:"type" validate:"omitempty"`

	OrderID int `json:"order_id" validate:"omitempty"`
}

type OrderDocumentDTO struct {
	ID   int    `json:"id"`
	Name string `json:"name"`
	Path string `json:"path"`
	Type string `json:"type"`

	OrderID int `json:"order_id"`

	CreatedAt string `json:"created_at"`
	UpdatedAt string `json:"updated_at"`
}

type ShortOrderDocumentDTO struct {
	ID      int    `json:"id"`
	Name    string `json:"name"`
	OrderID int    `json:"order_id"`
}
