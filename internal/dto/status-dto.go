package dto

// CreateStatusDTO: Что клиент присылает для создания.
type CreateStatusDTO struct {
	Name string `json:"name" validate:"required"`
	Type int    `json:"type" validate:"required"`
	Code string `json:"code" validate:"required"`
}

// UpdateStatusDTO: Что клиент может прислать для обновления.
type UpdateStatusDTO struct {
	ID        uint64  `json:"-"`
	IconSmall *string `json:"icon_small,omitempty"` // <-- Это для обновления URL вручную (если нужно)
	Name      *string `json:"name,omitempty"`
	Type      *int    `json:"type,omitempty"`
	Code      *string `json:"code,omitempty"`
	IconBig   *string `json:"icon_big,omitempty"` // <-- Это для обновления URL вручную (если нужно)
}

// StatusDTO: Что сервер отправляет клиенту в ответ.
type StatusDTO struct {
	ID        uint64 `json:"id"`
	IconSmall string `json:"icon_small,omitempty"`
	IconBig   string `json:"icon_big,omitempty"`
	Name      string `json:"name"`
	Type      int    `json:"type"`
	Code      string `json:"code"`
	CreatedAt string `json:"created_at"`
	UpdatedAt string `json:"updated_at,omitempty"`
}

type ShortStatusDTO struct {
	ID   uint64 `json:"id"`
	Name string `json:"name"`
}
