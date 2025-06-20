package dto

type CreateEquipmentTypeDTO struct {
	Name string `json:"name" validate:"required"`
}

type UpdateEquipmentTypeDTO struct {
	ID   int    `json:"id" validate:"required"`
	Name string `json:"name" validate:"omitempty"`
}

type EquipmentTypeDTO struct {
	ID        int    `json:"id"`
	Name      string `json:"name"`
	CreatedAt string `json:"created_at"`
	UpdatedAt string `json:"updated_at"`
}

type ShortEquipmentTypeDTO struct {
	ID   int    `json:"id"`
	Name string `json:"name"`
}
