package dto

// CreateStatusDTO: Что клиент присылает для создания.
type CreateStatusDTO struct {
	Name string `json:"name" validate:"required"`
	Type int    `json:"type" validate:"required"`
	Code string `json:"code" validate:"omitempty,uppercase,min=2"`
}

// UpdateStatusDTO: Что клиент может прислать для обновления.
type UpdateStatusDTO struct {
	IconSmall *string `json:"icon_small,omitempty"` // <-- Это для обновления URL вручную (если нужно)
	Name      *string `json:"name,omitempty" validate:"omitempty,min=1"`
	Type      *int    `json:"type,omitempty" validate:"omitempty,gte=0"`
	Code      *string `json:"code,omitempty,uppercase"`
	IconBig   *string `json:"icon_big,omitempty"` // <-- Это для обновления URL вручную (если нужно)
}

// StatusDTO: Что сервер отправляет клиенту в ответ.
type StatusDTO struct {
	ID        uint64 `json:"id"`
	IconSmall string `json:"icon_small,omitempty"`
	IconBig   string `json:"icon_big,omitempty"`
	Name      string `json:"name"`
	Type      int    `json:"type"`
	Code      string `json:"-"`
	CreatedAt string `json:"created_at"`
	UpdatedAt string `json:"updated_at,omitempty"`
}

type ShortStatusDTO struct {
	ID   uint64 `json:"id"`
	Name string `json:"name"`
}
