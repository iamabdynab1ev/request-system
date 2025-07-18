package dto



type CreateDepartmentDTO struct {
	Name     string `json:"name" validate:"required"`
	StatusID int    `json:"status_id" validate:"required"`
}

type UpdateDepartmentDTO struct {
	ID       int    `json:"id" validate:"required"`
	Name     string `json:"name" validate:"omitempty"`
	StatusID int    `json:"status_id" validate:"omitempty"`
}

type DepartmentDTO struct {
	ID        int    `json:"id"`
	Name      string `json:"name"`
	StatusID  int    `json:"status_id"`
	CreatedAt string `json:"created_at"`
	UpdatedAt string `json:"updated_at"`
}

type ShortDepartmentDTO struct {
	ID   int    `json:"id"`
	Name string `json:"name"`
}