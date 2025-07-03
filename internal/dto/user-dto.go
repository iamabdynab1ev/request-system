package dto

type CreateUserDTO struct {
	Fio          string `json:"fio" validate:"required"`
	Email        string `json:"email" validate:"required,email"`
	PhoneNumber  string `json:"phone_number" validate:"required"`
	Password     string `json:"password" validate:"required,min=8"`
	StatusID     int    `json:"status_id" validate:"required,gt=0"`
	RoleID       int    `json:"role_id" validate:"required,gt=0"`
	DepartmentID int    `json:"department_id" validate:"required,gt=0"`
	Position     string `json:"position,omitempty,"`
	BranchID     *int   `json:"branch_id,omitempty" validate:"omitempty,gt=0"`
	OfficeID     *int   `json:"office_id,omitempty" validate:"omitempty,gt=0"`
	OtdelID      *int   `json:"otdel_id,omitempty" validate:"omitempty,gt=0"`
}

type UpdateUserDTO struct {
	Fio          *string `json:"fio,omitempty"`
	Email        *string `json:"email,omitempty" validate:"omitempty,email"`
	PhoneNumber  *string `json:"phone_number,omitempty"`
	Password     *string `json:"password,omitempty" validate:"omitempty,min=8"`
	Position     *string `json:"position,omitempty"`
	StatusID     *int    `json:"status_id,omitempty" validate:"omitempty,gt=0"`
	DepartmentID *int    `json:"department_id,omitempty" validate:"omitempty,gt=0"`
	BranchID     *int    `json:"branch_id,omitempty" validate:"omitempty,gt=0"`
	OfficeID     *int    `json:"office_id,omitempty" validate:"omitempty,gt=0"`
	OtdelID      *int    `json:"otdel_id,omitempty" validate:"omitempty,gt=0"`
}

type UserDTO struct {
	ID          uint64             `json:"id"`
	FIO         string             `json:"fio"`
	Email       string             `json:"email"`
	PhoneNumber string             `json:"phone_number"`
	Position    string             `json:"position,omitempty"`
	Status      ShortStatusDTO     `json:"status"`
	Role        ShortRoleDTO       `json:"role"`
	Department  ShortDepartmentDTO `json:"department"`
	BranchID    *int64             `json:"branch_id,omitempty"`
	OfficeID    *int64             `json:"office_id,omitempty"`
	OtdelID     *int64             `json:"otdel_id,omitempty"`

	CreatedAt string `json:"created_at"`
	UpdatedAt string `json:"updated_at"`
}

type ShortUserDTO struct {
	ID  int    `json:"id"`
	Fio string `json:"fio"`
}
