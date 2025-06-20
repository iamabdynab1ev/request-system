package dto

type CreateEquipmentDTO struct {
	Name    		string `json:"name" validate:"required"`
	Address 		string `json:"address" validate:"required"`

	BranchID        int `json:"branch_id" validate:"required"`
	OfficeID        int `json:"office_id" validate:"required"`
	StatusID        int `json:"status_id" validate:"required"`
	EquipmentTypeID int `json:"equipment_type_id" validate:"required"`
}

type UpdateEquipmentDTO struct {
	ID      		int    `json:"id" validate:"required"`
	Name    		string `json:"name" validate:"omitempty"`
	Address 		string `json:"address" validate:"omitempty"`

	BranchID        int `json:"branch_id" validate:"omitempty"`
	OfficeID        int `json:"office_id" validate:"omitempty"`
	StatusID        int `json:"status_id" validate:"omitempty"`
	EquipmentTypeID int `json:"equipment_type_id" validate:"omitempty"`
}

type EquipmentDTO struct {
	ID      		int    `json:"id"`
	Name  		 	string `json:"name"`
	Address 		string `json:"address"`

	Branch        	ShortBranchDTO        `json:"branch"`
	Office        	ShortOfficeDTO        `json:"office"`
	EquipmentType 	ShortEquipmentTypeDTO `json:"equipment"`
	StatusID      	int                   `json:"status_id"`

	CreatedAt 		string `json:"created_at"`
	UpdatedAt 		string `json:"updated_at"`
}

type ShortEquipmentDTO struct {
	ID   		int    `json:"id"`
	Name 		string `json:"name"`
}
