package dto

type CreateEquipmentTypeDTO struct {
	Name string `json:"name" validate:"required,max=255"`
}

type UpdateEquipmentTypeDTO struct {
	Name string `json:"name" validate:"omitempty,min=1,max=255"`
}

type EquipmentTypeDTO struct {
	ID        uint64  `json:"id"`
	Name      string  `json:"name"`
	CreatedAt *string `json:"created_at"`
	UpdatedAt *string `json:"updated_at"`
}

type ShortEquipmentTypeDTO struct {
	ID   uint64 `json:"id"`
	Name string `json:"name"`
}
