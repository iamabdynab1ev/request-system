package dto

type CreatePriorityDTO struct {
	Name string `json:"name" validate:"required,max=50"`
	Code string `json:"code" validate:"omitempty,uppercase"`
	Rate int    `json:"rate" validate:"omitempty,gte=0"`
}
type UpdatePriorityDTO struct {
	IconSmall *string `json:"icon_small,omitempty"`
	IconBig   *string `json:"icon_big,omitempty"`
	Code      *string `json:"code,omitempty" validate:"omitempty"`
	Name      *string `json:"name,omitempty" validate:"omitempty,max=50"`
	Rate      *int    `json:"rate,omitempty"`
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
