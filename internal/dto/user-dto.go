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
	Fio          string  `json:"fio" validate:"omitempty"`
	Email        string  `json:"email" validate:"omitempty,email"`
	Position     string  `json:"position" validate:"omitempty"`
	PhoneNumber  string  `json:"phone_number" validate:"omitempty"`
	Password     string  `json:"password" validate:"omitempty"`
	StatusID     uint64  `json:"status_id" validate:"omitempty"`
	RoleID       uint64  `json:"role_id" validate:"omitempty"`
	BranchID     uint64  `json:"branch_id" validate:"omitempty"`
	DepartmentID uint64  `json:"department_id" validate:"omitempty"`
	OfficeID     *uint64 `json:"office_id" validate:"omitempty"`
	OtdelID      *uint64 `json:"otdel_id" validate:"omitempty"`
	PhotoURL     *string `json:"photo_url,omitempty"`
}

type UserDTO struct {
	ID          uint64         `json:"id"`
	Fio         string         `json:"fio"`
	Email       string         `json:"email"`
	Position    string         `json:"position"`
	PhoneNumber string         `json:"phone_number"`
	RoleID      uint64         `json:"role_id"`
	RoleName    string         `json:"role_name"`
	Branch      uint64         `json:"branch_id"`
	Department  uint64         `json:"department_id"`
	Office      *uint64        `json:"office_id"`
	Otdel       *uint64        `json:"otdel_id"`
	Status      ShortStatusDTO `json:"status_id"`
	PhotoURL    *string        `json:"photo_url"`
	CreatedAt   string         `json:"created_at"`
	UpdatedAt   string         `json:"updated_at"`
}

type ShortUserDTO struct {
	ID  uint64 `json:"id"`
	Fio string `json:"fio"`
}
