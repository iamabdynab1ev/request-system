package dto

type CreateUserDTO struct {
	Fio         string  `json:"fio" validate:"required,min=2"`
	Email       string  `json:"email" validate:"email,omitempty"`
	PhoneNumber string  `json:"phone_number" validate:"required"`
	PositionID  uint64  `json:"position_id" validate:"required"`
	StatusID    *uint64 `json:"status_id" validate:"omitempty"`

	RoleIDs []uint64 `json:"role_ids" validate:"required,dive,gte=1"`

	BranchID     *uint64 `json:"branch_id" validate:"omitempty"`
	DepartmentID *uint64 `json:"department_id" validate:"omitempty"`
	OfficeID     *uint64 `json:"office_id" validate:"omitempty"`
	OtdelID      *uint64 `json:"otdel_id" validate:"omitempty"`

	PhotoURL *string `json:"photo_url,omitempty"`
	IsHead   bool    `json:"is_head"`
}

type UpdateUserDTO struct {
	ID uint64 `json:"id" validate:"required"`

	Fio         *string `json:"fio" validate:"omitempty,min=2"`
	Email       *string `json:"email" validate:"omitempty,email"`
	PhoneNumber *string `json:"phone_number" validate:"omitempty"`
	Password    *string `json:"password" validate:"omitempty,min=6"`

	PositionID *uint64 `json:"position_id" validate:"omitempty"`
	StatusID   *uint64 `json:"status_id" validate:"omitempty"`

	RoleIDs *[]uint64 `json:"role_ids" validate:"omitempty,dive,gte=1"`

	BranchID     *uint64 `json:"branch_id" validate:"omitempty"`
	DepartmentID *uint64 `json:"department_id" validate:"omitempty"`
	OfficeID     *uint64 `json:"office_id" validate:"omitempty"`
	OtdelID      *uint64 `json:"otdel_id" validate:"omitempty"`

	PhotoURL *string `json:"photo_url,omitempty"`
	IsHead   *bool   `json:"is_head,omitempty"`
}

type UpdateUserPermissionsDTO struct {
	HasAccessIDs []uint64 `json:"has_access_ids"`
	NoAccessIDs  []uint64 `json:"no_access_ids"`
}

type UserDTO struct {
	ID          uint64  `json:"id"`
	Fio         string  `json:"fio"`
	Email       string  `json:"email"`
	PhoneNumber string  `json:"phone_number"`
	Username    *string `json:"username"`

	PositionID *uint64 `json:"position_id"`
	StatusID   uint64  `json:"status_id"`
	StatusCode string  `json:"status_code,omitempty"`

	BranchID       *uint64  `json:"branch_id"`
	DepartmentID   *uint64  `json:"department_id"`
	OfficeID       *uint64  `json:"office_id"`
	OtdelID        *uint64  `json:"otdel_id"`
	BranchName     *string  `json:"branch_name,omitempty"`
	DepartmentName *string  `json:"department_name,omitempty"`
	PositionName   *string  `json:"position_name,omitempty"`
	OtdelName      *string  `json:"otdel_name,omitempty"`
	OfficeName     *string  `json:"office_name,omitempty"`
	RoleIDs        []uint64 `json:"role_ids"`

	PhotoURL           *string `json:"photo_url,omitempty"`
	MustChangePassword bool    `json:"must_change_password"`
	IsHead             bool    `json:"is_head"`

	CreatedAt string `json:"created_at"`
	UpdatedAt string `json:"updated_at"`
}

type ShortUserDTO struct {
	ID   uint64 `json:"id"`
	Fio  string `json:"fio"`
	Role string `json:"role,omitempty"`
}
