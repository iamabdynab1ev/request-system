package dto

type CreateUserDTO struct {
	Fio          string   `json:"fio" validate:"required"`
	Email        string   `json:"email" validate:"email,omitempty"`
	PhoneNumber  string   `json:"phone_number" validate:"required"`
	Position     string   `json:"position" validate:"required"`
	StatusID     uint64   `json:"status_id" validate:"omitempty"`
	RoleIDs      []uint64 `json:"role_ids" validate:"required,dive,gte=1"`
	BranchID     uint64   `json:"branch_id" validate:"required"`
	DepartmentID uint64   `json:"department_id" validate:"required"`
	OfficeID     *uint64  `json:"office_id" validate:"omitempty"`
	OtdelID      *uint64  `json:"otdel_id" validate:"omitempty"`
	PhotoURL     *string  `json:"photo_url,omitempty"`
	IsHead       bool     `json:"is_head"`
}

type UpdateUserDTO struct {
	ID                  uint64    `json:"id" validate:"required"`
	Fio                 *string   `json:"fio" validate:"omitempty"`
	Email               *string   `json:"email" validate:"omitempty,email"`
	Position            *string   `json:"position" validate:"omitempty"`
	PhoneNumber         *string   `json:"phone_number" validate:"omitempty"`
	Password            *string   `json:"password" validate:"omitempty,min=6"`
	StatusID            *uint64   `json:"status_id" validate:"omitempty"`
	RoleIDs             *[]uint64 `json:"role_ids" validate:"omitempty,dive,gte=1"`
	DirectPermissionIDs *[]uint64 `json:"direct_permission_ids" validate:"omitempty,dive,gte=1"`
	DeniedPermissionIDs *[]uint64 `json:"denied_permission_ids" validate:"omitempty,dive,gte=1"`
	BranchID            *uint64   `json:"branch_id" validate:"omitempty"`
	DepartmentID        *uint64   `json:"department_id" validate:"omitempty"`
	OfficeID            *uint64   `json:"office_id" validate:"omitempty"`
	OtdelID             *uint64   `json:"otdel_id" validate:"omitempty"`
	PhotoURL            *string   `json:"photo_url,omitempty"`
	IsHead              *bool     `json:"is_head,omitempty"`
}

type UserDTO struct {
	ID                 uint64              `json:"id"`
	Fio                string              `json:"fio"`
	Email              string              `json:"email"`
	PhoneNumber        string              `json:"phone_number"`
	Position           string              `json:"position"`
	StatusID           uint64              `json:"status_id"`
	BranchID           uint64              `json:"branch_id"`
	DepartmentID       uint64              `json:"department_id"`
	RoleIDs            []uint64            `json:"role_ids"`
	OfficeID           *uint64             `json:"office_id,omitempty"`
	OtdelID            *uint64             `json:"otdel_id,omitempty"`
	PhotoURL           *string             `json:"photo_url,omitempty"`
	MustChangePassword bool                `json:"must_change_password"`
	IsHead             bool                `json:"is_head"`
	Permissions        *PermissionsInfoDTO `json:"permissions,omitempty"`
}

type PermissionsInfoDTO struct {
	CurrentPermissionIDs     []uint64 `json:"current_permission_ids"`
	UnavailablePermissionIDs []uint64 `json:"unavailable_permission_ids"`
}
