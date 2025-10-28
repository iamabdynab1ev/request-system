package dto

import "github.com/aarondl/null/v8"

type CreatePositionDTO struct {
	Name         string   `json:"name" validate:"required"`
	DepartmentID null.Int `json:"department_id"`
	OtdelID      null.Int `json:"otdel_id"`
	BranchID     null.Int `json:"branch_id"`
	OfficeID     null.Int `json:"office_id"`
	Type         *string  `json:"type" validate:"omitempty"`
	StatusID     int      `json:"status_id" validate:"required"`
}

type UpdatePositionDTO struct {
	Name         *string  `json:"name"`
	DepartmentID null.Int `json:"department_id"`
	OtdelID      null.Int `json:"otdel_id"`
	BranchID     null.Int `json:"branch_id"`
	OfficeID     null.Int `json:"office_id"`
	Type         *string  `json:"type"`
	StatusID     *int     `json:"status_id"`
}

type PositionResponseDTO struct {
	ID           uint64   `json:"id"`
	Name         string   `json:"name"`
	DepartmentID null.Int `json:"department_id,omitempty"`
	OtdelID      null.Int `json:"otdel_id,omitempty"`
	BranchID     null.Int `json:"branch_id,omitempty"`
	OfficeID     null.Int `json:"office_id,omitempty"`
	Type         *string  `json:"type,omitempty"`
	StatusID     int      `json:"status_id"`
	CreatedAt    string   `json:"created_at"`
	UpdatedAt    string   `json:"updated_at"`
}

type ShortPositionDTO struct {
	ID   uint64 `json:"id"`
	Name string `json:"name"`
}
type PositionTypeDTO struct {
	Code string `json:"code"`
	Name string `json:"name"`
}
