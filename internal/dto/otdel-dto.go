package dto

type CreateOtdelDTO struct {
	Name          string `json:"name" validate:"required"`
	StatusID      uint64 `json:"status_id" validate:"required"`
	DepartmentsID uint64 `json:"department_id" validate:"required"`
}

type UpdateOtdelDTO struct {
	ID            uint64 `json:"id" validate:"required"`
	Name          string `json:"name" validate:"omitempty"`
	StatusID      uint64 `json:"status_id" validate:"omitempty"`
	DepartmentsID uint64 `json:"department_id" validate:"omitempty"`
}

type OtdelDTO struct {
	ID            uint64 `json:"id"`
	Name          string `json:"name"`
	StatusID      uint64 `json:"status_id"`
	DepartmentsID uint64 `json:"department_id"`
	CreatedAt     string `json:"created_at"`
	UpdatedAt     string `json:"updated_at"`
}

type ShortOtdelDTO struct {
	ID   uint64 `json:"id"`
	Name string `json:"name"`
}
