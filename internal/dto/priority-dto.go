package dto

type CreatePriorityDTO struct {
	IconSmall string `json:"icon_small" validate:"omitempty"`
	IconBig   string `json:"icon_big" validate:"omitempty"`
	Code      string `json:"code" validate:"required"`
	Name      string `json:"name" validate:"required,max=50"`
	Rate      int    `json:"rate" validate:"required"`
}

type UpdatePriorityDTO struct {
	ID        uint64 `json:"id" validate:"required"`
	IconSmall string `json:"icon_small" validate:"omitempty"`
	IconBig   string `json:"icon_big" validate:"omitempty"`
	Code      string `json:"code" validate:"omitempty"`
	Name      string `json:"name" validate:"omitempty"`
	Rate      int    `json:"rate" validate:"omitempty"`
}

type PriorityDTO struct {
	ID        uint64 `json:"id"`
	IconBig   string `json:"icon_big"`
	IconSmall string `json:"icon_small"`
	Name      string `json:"name"`
	Code      string `json:"code"`
	Rate      int    `json:"rate"`
	CreatedAt string `json:"created_at"`

	UpdatedAt string `json:"updated_at"`
}

type ShortPriorityDTO struct {
	ID   uint64 `json:"id"`
	Name string `json:"name"`
}
