package dto

type CreateDepartmentDTO struct {
	Name     string `json:"name" validate:"required"`
	StatusID uint64 `json:"status_id" validate:"required"`
}

type UpdateDepartmentDTO struct {
	ID       uint64 `json:"id" validate:"required"`
	Name     string `json:"name" validate:"omitempty"`
	StatusID uint64 `json:"status_id" validate:"omitempty"`
}

type DepartmentDTO struct {
	ID        uint64 `json:"id"`
	Name      string `json:"name"`
	StatusID  uint64 `json:"status_id"`
	CreatedAt string `json:"created_at"`
	UpdatedAt string `json:"updated_at"`
}

type ShortDepartmentDTO struct {
	ID   uint64 `json:"id"`
	Name string `json:"name"`
}
