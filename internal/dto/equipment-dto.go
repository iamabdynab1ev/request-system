package dto

type CreateEquipmentDTO struct {
	Name    string `json:"name" validate:"required"`
	Address string `json:"address" validate:"required"`

	BranchID        uint64 `json:"branch_id" validate:"required"`
	OfficeID        uint64 `json:"office_id" validate:"required"`
	StatusID        uint64 `json:"status_id" validate:"required"`
	EquipmentTypeID uint64 `json:"equipment_type_id" validate:"required"`
}

type UpdateEquipmentDTO struct {
	Name            *string `json:"name,omitempty"           validate:"omitempty"`
	Address         *string `json:"address,omitempty"        validate:"omitempty"`
	BranchID        *uint64 `json:"branch_id,omitempty"      validate:"omitempty,gt=0"`
	OfficeID        *uint64 `json:"office_id,omitempty"      validate:"omitempty,gt=0"`
	StatusID        *uint64 `json:"status_id,omitempty"      validate:"omitempty,gt=0"`
	EquipmentTypeID *uint64 `json:"equipment_type_id,omitempty" validate:"omitempty,gt=0"`
}

type EquipmentDTO struct {
	ID      uint64 `json:"id"`
	Name    string `json:"name"`
	Address string `json:"address"`

	Branch        ShortBranchDTO        `json:"branch"`
	Office        ShortOfficeDTO        `json:"office"`
	EquipmentType ShortEquipmentTypeDTO `json:"equipment"`
	StatusID      uint64                `json:"status_id"`

	CreatedAt string `json:"created_at"`
	UpdatedAt string `json:"updated_at"`
}
type EquipmentListResponseDTO struct {
	ID              uint64 `json:"id"`
	Name            string `json:"name"`
	Address         string `json:"address"`
	BranchID        uint64 `json:"branch_id"`
	OfficeID        uint64 `json:"office_id"`
	EquipmentTypeID uint64 `json:"equipment_type_id"`
	StatusID        uint64 `json:"status_id"`
	CreatedAt       string `json:"created_at"`
	UpdatedAt       string `json:"updated_at"`
}
type ShortEquipmentDTO struct {
	ID   uint64 `json:"id"`
	Name string `json:"name"`
}
