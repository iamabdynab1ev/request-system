package dto

type CreateEquipmentTypeDTO struct {
	Name string `json:"name" validate:"required,max=50"`
}

type UpdateEquipmentTypeDTO struct {
	ID   int    `json:"id" validate:"required"`
	Name string `json:"name" validate:"required,max=50"`
}

type EquipmentTypeDTO struct {
	ID        int    `json:"id"`
	Name      string `json:"name"`
	CreatedAt string `json:"created_at"`
}

type ShortEquipmentTypeDTO struct {
	ID   int    `json:"id"`
	Name string `json:"name"`
}

