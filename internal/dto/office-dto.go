package dto

import "time"

type CreateOfficeDTO struct {
	Name     string  `json:"name" validate:"required"`
	Address  string  `json:"address" validate:"required"`
	OpenDate string  `json:"open_date" validate:"required,datetime=2006-01-02"`
	StatusID uint64  `json:"status_id" validate:"required,gt=0"`
	BranchID *uint64 `json:"branch_id" validate:"omitempty,required_without=ParentID,gt=0"`
	ParentID *uint64 `json:"office_id" validate:"omitempty,required_without=BranchID,gt=0"`
}

type UpdateOfficeDTO struct {
	Name     *string `json:"name" validate:"omitempty"`
	Address  *string `json:"address" validate:"omitempty"`
	OpenDate *string `json:"open_date" validate:"omitempty,datetime=2006-01-02"`
	BranchID *uint64 `json:"branch_id" validate:"omitempty,gt=0"`
	StatusID *uint64 `json:"status_id" validate:"omitempty,gt=0"`
	ParentID *uint64 `json:"office_id" validate:"omitempty,gt=0"`
}

type OfficeDTO struct {
	ID         uint64    `json:"id"`
	Name       string    `json:"name"`
	Address    string    `json:"address"`
	OpenDate   time.Time `json:"open_date"`
	BranchID   *uint64   `json:"branch_id,omitempty"`
	BranchName *string   `json:"branch_name,omitempty"`
	ParentID   *uint64   `json:"office_id,omitempty"`
	ParentName *string   `json:"parent_name,omitempty"`
	StatusID   uint64    `json:"status_id"`
	StatusName string    `json:"status_name"`
	CreatedAt  time.Time `json:"created_at"`
	UpdatedAt  time.Time `json:"updated_at"`
}

// OfficeListResponseDTO - для списков, где не нужны вложенные объекты
type OfficeListResponseDTO struct {
	ID        uint64  `json:"id"`
	Name      string  `json:"name"`
	Address   string  `json:"address"`
	OpenDate  string  `json:"open_date"`
	BranchID  *uint64 `json:"branch_id"`
	ParentID  *uint64 `json:"office_id"`
	StatusID  uint64  `json:"status_id"`
	CreatedAt string  `json:"created_at"`
}
