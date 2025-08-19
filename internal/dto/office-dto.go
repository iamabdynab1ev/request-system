package dto

import "time"

type CreateOfficeDTO struct {
	Name     string `json:"name" validate:"required"`
	Address  string `json:"address" validate:"required"`
	OpenDate string `json:"open_date" validate:"required,datetime=2006-01-02"`
	BranchID uint64 `json:"branch_id" validate:"required,gt=0"`
	StatusID uint64 `json:"status_id" validate:"required,gt=0"`
}

type UpdateOfficeDTO struct {
	Name     string `json:"name" validate:"omitempty"`
	Address  string `json:"address" validate:"omitempty"`
	OpenDate string `json:"open_date" validate:"omitempty,datetime=2006-01-02"`
	BranchID uint64 `json:"branch_id" validate:"omitempty,gt=0"`
	StatusID uint64 `json:"status_id" validate:"omitempty,gt=0"`
}

type OfficeDTO struct {
	ID        uint64
	Name      string
	Address   string
	OpenDate  time.Time
	Branch    *ShortBranchDTO // Объект филиала
	Status    *ShortStatusDTO // Объект статуса
	CreatedAt time.Time
	UpdatedAt time.Time
}

type OfficeResponseDTO struct {
	ID        uint64 `json:"id"`
	Name      string `json:"name"`
	Address   string `json:"address"`
	OpenDate  string `json:"open_date"`
	BranchID  uint64 `json:"branch_id"` 
	StatusID  uint64 `json:"status_id"` 
	CreatedAt string `json:"created_at"`
	UpdatedAt string `json:"updated_at"`
}

type ShortOfficeDTO struct {
	ID   uint64 `json:"id"`
	Name string `json:"name"`
}
