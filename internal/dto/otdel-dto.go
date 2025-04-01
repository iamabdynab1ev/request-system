package dto

type CreateOtdelDTO struct {
	Name         string `json:"name" validate:"required,max=50"`
	StatusID     int    `json:"status_id" validate:"required"`
	DepartmentID int    `json:"department_id" validate:"required"`
}

type UpdateOtdelDTO struct {
	ID           int    `json:"id" validate:"required"`
	Name         string `json:"name" validate:"omitempty,max=50"`
	StatusID     int    `json:"status_id" validate:"omitempty"`
	DepartmentID int    `json:"department_id" validate:"omitempty"`
}


type OtdelDTO struct {
	ID          int                `json:"id"`
	Name        string             `json:"name"`
	Status      ShortStatusDTO     `json:"status"`
	Department  ShortDepartmentDTO `json:"department"`
	CreatedAt   string             `json:"created_at"`
}

type ShortOtdelDTO struct {
	ID   int    `json:"id"`
	Name string `json:"name"`
}
