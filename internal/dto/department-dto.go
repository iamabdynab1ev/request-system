package dto

type CreateDepartmentDTO struct {
	Name     string `json:"name" validate:"required"`
	StatusID uint64 `json:"status_id" validate:"required"`
}

type UpdateDepartmentDTO struct {
	Name     *string `json:"name" validate:"omitempty,min=1"`
	StatusID *uint64 `json:"status_id" validate:"omitempty,gt=0"`
}

type DepartmentDTO struct {
	ID        uint64 `json:"id"`
	Name      string `json:"name"`
	StatusID  uint64 `json:"status_id"`
	CreatedAt string `json:"created_at"`
	UpdatedAt string `json:"updated_at"`
}
type DepartmentStatsDTO struct {
	ID                uint64 `json:"id"`
	Name              string `json:"name"`
	OpenOrdersCount   int64  `json:"open"`
	ClosedOrdersCount int64  `json:"closed"`
}
type ShortDepartmentDTO struct {
	ID   uint64 `json:"id"`
	Name string `json:"name"`
}
