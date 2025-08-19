package dto

type CreateOtdelDTO struct {
	Name          string `json:"name" validate:"required"`
	StatusID      int    `json:"status_id" validate:"required"`
	DepartmentsID int    `json:"department_id" validate:"required"`
}

type UpdateOtdelDTO struct {
	ID            int    `json:"id" validate:"required"`
	Name          string `json:"name" validate:"omitempty"`
	StatusID      int    `json:"status_id" validate:"omitempty"`
	DepartmentsID int    `json:"department_id" validate:"omitempty"`
}

type OtdelDTO struct {
	ID            int    `json:"id"`
	Name          string `json:"name"`
	StatusID      int    `json:"status_id"`
	DepartmentsID int    `json:"department_id"`
	CreatedAt string `json:"created_at"`
	UpdatedAt string `json:"updated_at"`
}

type ShortOtdelDTO struct {
	ID   int    `json:"id"`
	Name string `json:"name"`
}
