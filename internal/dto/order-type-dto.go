// Файл: internal/dto/order-type-dto.go

package dto

// CreateOrderTypeDTO используется для создания нового типа заявки.
type CreateOrderTypeDTO struct {
	Name     string  `json:"name" validate:"required"`
	Code     *string `json:"code" validate:"omitempty,uppercase,min=2"`
	StatusID int     `json:"status_id" validate:"required"`
}

// UpdateOrderTypeDTO используется для обновления существующего типа заявки.
type UpdateOrderTypeDTO struct {
	Name     *string `json:"name,omitempty" validate:"omitempty,min=1"`
	Code     *string `json:"code,omitempty" validate:"omitempty,uppercase"`
	StatusID *int    `json:"status_id,omitempty"`
}

// OrderTypeResponseDTO используется для отправки данных о типе заявки клиенту.
type OrderTypeResponseDTO struct {
	ID        uint64 `json:"id"`
	Name      string `json:"name"`
	Code      string `json:"code,omitempty"`
	StatusID  int    `json:"status_id"`
	CreatedAt string `json:"created_at"`
	UpdatedAt string `json:"updated_at,omitempty"`
}
