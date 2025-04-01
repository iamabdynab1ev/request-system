package dto

type CreateOrderDTO struct {
	Name         string `json:"name" validate:"required,max=50"`
	DepartmentID int    `json:"department_id" validate:"required"`
	OtdelID      int    `json:"otdel_id" validate:"required"`
	ProretyID    int    `json:"prorety_id" validate:"required"`
	StatusID     int    `json:"status_id" validate:"required"`
	BranchID     int    `json:"branch_id" validate:"required"`
	OfficeID     int    `json:"office_id" validate:"required"`
	EquipmentID  int    `json:"equipment_id" validate:"required"`
	UserID       int    `json:"user_id" validate:"required"`
	Duration     string `json:"duration" validate:"required"`
	Address      string `json:"address" validate:"required"`
}

type UpdateOrderDTO struct {
	ID           int    `json:"id" validate:"required"`
	Name         string `json:"name" validate:"omitempty,max=50"`
	DepartmentID int    `json:"department_id" validate:"omitempty"`
	OtdelID      int    `json:"otdel_id" validate:"omitempty"`
	ProretyID    int    `json:"prorety_id" validate:"omitempty"`
	StatusID     int    `json:"status_id" validate:"omitempty"`
	BranchID     int    `json:"branch_id" validate:"omitempty"`
	OfficeID     int    `json:"office_id" validate:"omitempty"`
	EquipmentID  int    `json:"equipment_id" validate:"omitempty"`
	UserID       int    `json:"user_id" validate:"omitempty"`
	Duration     string `json:"duration" validate:"omitempty"`
	Address      string `json:"address" validate:"omitempty"`
}


type OrderDTO struct {
	ID             int                   `json:"id"`
	Name           string                `json:"name"`
	DepartmentID   ShortDepartmentDTO    `json:"department_id"`
	OtdelID        ShortOtdelDTO         `json:"otdel_id"`
	ProretyID      ShortProretyDTO       `json:"prorety_id"`
	StatusID       ShortStatusDTO        `json:"status_id"`
	BranchID       ShortBranchDTO        `json:"branch_id"`
	OfficeID       ShortOfficeDTO        `json:"office_id"`
	EquipmentID    ShortEquipmentDTO     `json:"equipment_id"`
	UserID         ShortUserDTO          `json:"user_id"`
	Duration       string                `json:"duration"`
	Address        string                `json:"address"`
	CreatedAt      string                `json:"created_at"`
}

type ShortOrderDTO struct {
	ID   int    `json:"id"`
	Name string `json:"name"`
}

