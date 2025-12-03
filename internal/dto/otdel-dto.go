// Файл: internal/dto/otdel_dto.go (предположительно)

package dto

type CreateOtdelDTO struct {
	Name          string  `json:"name" validate:"required"`
	StatusID      uint64  `json:"status_id" validate:"required"`
	DepartmentsID *uint64 `json:"department_id" validate:"omitempty,required_without=BranchID"`
	BranchID      *uint64 `json:"branch_id" validate:"omitempty,required_without=DepartmentsID"`
	ParentID      *uint64 `json:"otdel_id" validate:"omitempty,gt=0"`
}

type UpdateOtdelDTO struct {
	Name          string  `json:"name"`
	StatusID      uint64  `json:"status_id"`
	DepartmentsID *uint64 `json:"department_id"`
	BranchID      *uint64 `json:"branch_id"`
	ParentID      *uint64 `json:"otdel_id"`
}

type OtdelDTO struct {
	ID            uint64  `json:"id"`
	Name          string  `json:"name"`
	StatusID      uint64  `json:"status_id"`
	DepartmentsID *uint64 `json:"department_id,omitempty"`
	BranchID      *uint64 `json:"branch_id,omitempty"`
	ParentID      *uint64 `json:"otdel_id,omitempty"`
	CreatedAt     string  `json:"created_at"`
	UpdatedAt     string  `json:"updated_at"`
}

type ShortOtdelDTO struct {
	ID   uint64 `json:"id"`
	Name string `json:"name"`
}
