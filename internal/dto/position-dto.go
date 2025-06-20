package dto

type PositionDTO struct {
	ID        int    `json:"id"`
	Name      string `json:"name"`
	CreatedAt string `json:"created_at"`
	UpdatedAt string `json:"updated_at"`
}

type CreatePositionDTO struct {
	Name string `json:"name" validate:"required"`
}

type UpdatePositionDTO struct {
	Name string `json:"name" validate:"required"`
}

type ShortPositionDTO struct {
	Name string `json:"name"`
}
