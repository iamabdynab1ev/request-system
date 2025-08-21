package dto

type CreateStatusDTO struct {
	Name string `json:"name" validate:"required"`
	Type int    `json:"type" validate:"required"`
	Code string `json:"code" validate:"omitempty,uppercase,min=2"`
}

type UpdateStatusDTO struct {
	IconSmall *string `json:"icon_small,omitempty"`
	Name      *string `json:"name,omitempty" validate:"omitempty,min=1"`
	Type      *int    `json:"type,omitempty" validate:"omitempty,gte=0"`
	Code      *string `json:"code,omitempty" validate:"omitempty,uppercase"`
	IconBig   *string `json:"icon_big,omitempty"`
}

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
