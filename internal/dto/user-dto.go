package dto

type CreateUserDTO struct {
	Fio          string  `json:"fio" validate:"required"`
	Email        string  `json:"email" validate:"email,omitempty"`
	PhoneNumber  string  `json:"phone_number" validate:"required"`
	Position     string  `json:"position" validate:"required"`
	Password     string  `json:"password" validate:"required"`
	StatusID     uint64  `json:"status_id" validate:"required"`
	RoleID       uint64  `json:"role_id" validate:"required"`
	BranchID     uint64  `json:"branch_id" validate:"required"`
	DepartmentID uint64  `json:"department_id" validate:"required"`
	OfficeID     *uint64 `json:"office_id" validate:"omitempty"`
	OtdelID      *uint64 `json:"otdel_id" validate:"omitempty"`
	PhotoURL     *string `json:"photo_url,omitempty"`
}

type UpdateUserDTO struct {
	ID           uint64  `json:"id" validate:"required"`
	Fio          *string `json:"fio" validate:"omitempty"`
	Email        *string `json:"email" validate:"omitempty,email"`
	Position     *string `json:"position" validate:"omitempty"`
	PhoneNumber  *string `json:"phone_number" validate:"omitempty"`
	Password     *string `json:"password" validate:"omitempty"`
	StatusID     *uint64 `json:"status_id" validate:"omitempty"`
	RoleID       *uint64 `json:"role_id" validate:"omitempty"`
	BranchID     *uint64 `json:"branch_id" validate:"omitempty"`
	DepartmentID *uint64 `json:"department_id" validate:"omitempty"`
	OfficeID     *uint64 `json:"office_id" validate:"omitempty"`
	OtdelID      *uint64 `json:"otdel_id" validate:"omitempty"`
	PhotoURL     *string `json:"photo_url,omitempty"`
}

type UserDTO struct {
	ID           uint64  `json:"id"`
	Fio          string  `json:"fio"`
	Email        string  `json:"email"`
	PhoneNumber  string  `json:"phone_number"`
	Position     string  `json:"position"`
	RoleID       uint64  `json:"role_id"`
	StatusID     uint64  `json:"status_id"`
	BranchID     uint64  `json:"branch_id"`
	DepartmentID uint64  `json:"department_id"`
	OfficeID     *uint64 `json:"office_id,omitempty"`
	OtdelID      *uint64 `json:"otdel_id,omitempty"`
	RoleName     string  `json:"role_name"`
	PhotoURL     *string `json:"photo_url,omitempty"`
}
