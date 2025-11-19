package dto

type CreateUserDTO struct {
	Fio          string   `json:"fio" validate:"required"`
	Email        string   `json:"email" validate:"email,omitempty"`
	PhoneNumber  string   `json:"phone_number" validate:"required"`
	PositionID   uint64   `json:"position_id" validate:"required"`
	StatusID     uint64   `json:"status_id" validate:"omitempty"`
	RoleIDs      []uint64 `json:"role_ids" validate:"required,dive,gte=1"`
	BranchID     *uint64  `json:"branch_id" validate:"omitempty"`
	DepartmentID *uint64  `json:"department_id" validate:"omitempty"`
	OfficeID     *uint64  `json:"office_id" validate:"omitempty"`
	OtdelID      *uint64  `json:"otdel_id" validate:"omitempty"`
	PhotoURL     *string  `json:"photo_url,omitempty"`
	IsHead       bool     `json:"is_head"`
}

type UpdateUserDTO struct {
	ID           uint64    `json:"id" validate:"required"`
	Fio          *string   `json:"fio" validate:"omitempty"`
	Email        *string   `json:"email" validate:"omitempty,email"`
	PositionID   *uint64   `json:"position_id" validate:"omitempty"`
	PhoneNumber  *string   `json:"phone_number" validate:"omitempty"`
	Password     *string   `json:"password" validate:"omitempty,min=6"`
	StatusID     *uint64   `json:"status_id" validate:"omitempty"`
	RoleIDs      *[]uint64 `json:"role_ids" validate:"omitempty,dive,gte=1"`
	BranchID     *uint64   `json:"branch_id" validate:"omitempty"`
	DepartmentID *uint64   `json:"department_id" validate:"omitempty"`
	OfficeID     *uint64   `json:"office_id" validate:"omitempty"`
	OtdelID      *uint64   `json:"otdel_id" validate:"omitempty"`
	PhotoURL     *string   `json:"photo_url,omitempty"`
	IsHead       *bool     `json:"is_head,omitempty"`
}
type UpdateUserPermissionsDTO struct {
	HasAccessIDs []uint64 `json:"has_access_ids"`
	NoAccessIDs  []uint64 `json:"no_access_ids"`
}

type UserDTO struct {
	ID                 uint64   `json:"id"`
	Fio                string   `json:"fio"`
	Email              string   `json:"email"`
	PhoneNumber        string   `json:"phone_number"`
	PositionID         *uint64  `json:"position_id"`
	StatusID           uint64   `json:"status_id"`
	BranchID           *uint64  `json:"branch_id,omitempty"`
	DepartmentID       *uint64  `json:"department_id,omitempty"`
	OfficeID           *uint64  `json:"office_id,omitempty"`
	OtdelID            *uint64  `json:"otdel_id,omitempty"`
	RoleIDs            []uint64 `json:"role_ids"`
	PhotoURL           *string  `json:"photo_url,omitempty"`
	MustChangePassword bool     `json:"must_change_password"`
	IsHead             bool     `json:"is_head"`
}
type ShortUserDTO struct {
	ID   uint64 `json:"id"`
	Fio  string `json:"fio"`
	Role string `json:"role,omitempty"`
}
