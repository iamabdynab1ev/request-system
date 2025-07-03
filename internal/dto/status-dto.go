package dto

type CreateStatusDTO struct {
	Icon string `json:"icon" validate:"required"`
	Name string `json:"name" validate:"required,max=50"`
	Type int    `json:"type" validate:"required"`
	Code string `json:"code" validate:"required"`
}

type UpdateStatusDTO struct {
	ID   int    `json:"id" validate:"required"`
	Icon string `json:"icon" validate:"omitempty"`
	Name string `json:"name" validate:"omitempty"`
	Type int    `json:"type" validate:"omitempty"`
	Code string `json:"code" validate:"omitempty"`
}

type StatusDTO struct {
	ID        int    `json:"id"`
	Icon      string `json:"icon"`
	Name      string `json:"name"`
	Type      int    `json:"type"`
	Code      string `json:"code"`
	CreatedAt string `json:"created_at"`
}

type ShortStatusDTO struct {
	ID   int    `json:"id"`
	Name string `json:"name"`
}
