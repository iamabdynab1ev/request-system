package dto

type CreateUserDTO struct {
	Fio          string `json:"fio" validate:"required,min=3,max=255"`
	Email        string `json:"email" validate:"email,omitempty"`
	PhoneNumber  string `json:"phone_number" validate:"required,len=12,regexp=^[0-9]{12}$"`
	RoleID       int    `json:"role_id" validate:"required"`
	BranchID     int    `json:"branch_id" validate:"required"`
	DepartmentID int    `json:"department_id" validate:"required"`
	OfficeID     *int   `json:"office_id" validate:"omitempty"`
	OtdelID      *int   `json:"otdel_id" validate:"omitempty"`
}

type UpdateUserDTO struct {
	ID           int    `json:"id" validate:"required"`
	Fio          string `json:"fio" validate:"required,min=3,max=255"`
	Email        string `json:"email" validate:"email,omitempty"`
	PhoneNumber  string `json:"phone_number" validate:"required,len=12,regexp=^[0-9]{12}$"`
	RoleID       int    `json:"role_id" validate:" omitempty"`
	BranchID     int    `json:"branch_id" validate:"omitempty"`
	DepartmentID int    `json:"department_id" validate:"omitempty"`
	OfficeID     *int   `json:"office_id" validate:"omitempty"`
	OtdelID      *int   `json:"otdel_id" validate:"omitempty"`
	StatusID     *int   `json:"status_id" validate:"omitempty"`
}

type UserDTO struct {
	ID          int                 `json:"id"`
	Fio         string              `json:"fio"`
	Email       string              `json:"email"`
	PhoneNumber string              `json:"phone_number"`
	RoleID       int                `json:"role"`
	BranchID     int                `json:"branch"`
	DepartmentID int                `json:"department"`
	OfficeID     *int               `json:"office"`
	OtdelID      *int               `json:"otdel"`
	StatusID      ShortStatusDTO    `json:"status"`
	CreatedAt    string             `json:"created_at"`
}

type ShortUserDTO struct {
	ID  int    `json:"id"`
	Fio string `json:"fio"`
}
