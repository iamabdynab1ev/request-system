package dto

type CreateStatusDTO struct {
	Icon      string `json:"icon" validate:"required"`
	Name      string `json:"name" validate:"required,max=50"`
	Type      int    `json:"type" validate:"required"`
}

type UpdateStatusDTO struct {
	ID        int    `json:"id" validate:"required"`
	Icon      string `json:"icon" validate:"omitemty"`
	Name      string `json:"name" validate:"omitemty"`
	Type      int    `json:"type" validate:"omitemty"`
}

type StatusDTO struct {
	ID        int    `json:"id"`
	Icon      string `json:"icon"`
	Name      string `json:"name"`
	Type      int    `json:"type"`
	CreatedAt string `json:"created_at"`
}

type ShortStatusDTO struct {
	ID        int    `json:"id"`
	Name      string `json:"name"`
}