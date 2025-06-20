package dto

type CreateOrderDTO struct {
	Name         string `json:"name" validate:"required"`
	DepartmentID int    `json:"department_id" validate:"required"`
	OtdelID      int    `json:"otdel_id" validate:"required"`
	ProretyID    int    `json:"prorety_id" validate:"required"`
	StatusID     int    `json:"status_id" validate:"required"`
	BranchID     int    `json:"branch_id" validate:"required"`
	OfficeID     int    `json:"office_id" validate:"required"`
	EquipmentID  int    `json:"equipment_id" validate:"required"`
	ExecutorID   int    `json:"executor_id" validate:"required"`
	Duration     string `json:"duration" validate:"required"`
	Address      string `json:"address" validate:"required"`
	Massage      string `json:"message" validate:"required"`
}

type UpdateOrderDTO struct {
	ID           int    `json:"id" validate:"required"`
	Name         string `json:"name" validate:"omitempty"`
	DepartmentID int    `json:"department_id" validate:"omitempty"`
	OtdelID      int    `json:"otdel_id" validate:"omitempty"`
	ProretyID    int    `json:"prorety_id" validate:"omitempty"`
	StatusID     int    `json:"status_id" validate:"omitempty"`
	BranchID     int    `json:"branch_id" validate:"omitempty"`
	OfficeID     int    `json:"office_id" validate:"omitempty"`
	EquipmentID  int    `json:"equipment_id" validate:"omitempty"`
	ExecutorID   int    `json:"executor_id" validate:"omitempty"`
	Duration     string `json:"duration" validate:"omitempty"`
	Address      string `json:"address" validate:"omitempty"`
}

type OrderDTO struct {
	ID           int             `json:"id"`
	Name         string          `json:"name"`
	DepartmentID int             `json:"department_id"`
	OtdelID      int             `json:"otdel_id"`
	Prorety      ShortProretyDTO `json:"prorety"`
	Status       ShortStatusDTO  `json:"status"`
	Creator      ShortUserDTO    `json:"creator"`
	Executor     *ShortUserDTO   `json:"executor,omitempty"`
	BranchID     int             `json:"branch_id"`
	OfficeID     int             `json:"office_id"`
	EquipmentID  int             `json:"equipment_id"`
	Duration     string          `json:"duration"`
	Address      string          `json:"address"`
	CreatedAt    string          `json:"created_at"`
}

type ShortOrderDTO struct {
	ID   int    `json:"id"`
	Name string `json:"name"`
}
