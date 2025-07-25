package dto

type CreateStatusDTO struct {
	ID        int    `json:"id" validate:"required"`
	IconSmall string `json:"icon_small" validate:"required"`
	Name      string `json:"name" validate:"required"`
	Type      int    `json:"type" validate:"required"`
	Code      string `json:"code" validate:"required"`
	IconBig   string `json:"icon_big" validate:"required"`
}

type UpdateStatusDTO struct {
	ID        int    `json:"id" validate:"required"`
	IconSmall string `json:"icon_small" validate:"omitempty"`
	Name      string `json:"name" validate:"omitempty"`
	Type      int    `json:"type" validate:"omitempty"`
	Code      string `json:"code" validate:"omitempty"`
	IconBig   string `json:"icon_big" validate:"omitempty"`
}

type StatusDTO struct {
	ID        int     `json:"id"`
	IconSmall *string `json:"icon_small"`
	IconBig   *string `json:"icon_big"`
	Name      string  `json:"name"`
	Type      int     `json:"type"`
	Code      *string `json:"code"`

	CreatedAt string `json:"created_at"`
	UpdatedAt string `json:"updated_at"`
	DeletedAt string `json:"deleted_at"`
}

type ShortStatusDTO struct {
	ID   int    `json:"id"`
	Name string `json:"name"`
	Code string `json:"code"`
}
