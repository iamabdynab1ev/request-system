package dto

type CreateUserDTO struct {
	Fio          string `json:"fio" validate:"required"`
	Email        string `json:"email" validate:"email,omitempty"`
	PhoneNumber  string `json:"phone_number" validate:"required"`
	Position     string `json:"position" validate:"required"`
	Password     string `json:"password" validate:"required"`
	StatusID     int    `json:"status_id" validate:"required"`
	RoleID       int    `json:"role_id" validate:"required"`
	BranchID     int    `json:"branch_id" validate:"required"`
	DepartmentID int    `json:"department_id" validate:"required"`
	OfficeID     *int   `json:"office_id" validate:"omitempty"`
	OtdelID      *int   `json:"otdel_id" validate:"omitempty"`
}

type UpdateUserDTO struct {
	ID           int    `json:"id" validate:"required"`
	Fio          string `json:"fio" validate:"omitempty"`
	Email        string `json:"email" validate:"email,omitempty"`
	Position     string `json:"position" validate:"omitempty"`
	PhoneNumber  string `json:"phone_number" validate:"omitempty"`
	Password     string `json:"password" validate:"omitempty"`
	StatusID     int    `json:"status_id" validate:"omitempty"`
	RoleID       int    `json:"role_id" validate:"omitempty"`
	BranchID     int    `json:"branch_id" validate:"omitempty"`
	DepartmentID int    `json:"department_id" validate:"omitempty"`
	OfficeID     *int   `json:"office_id" validate:"omitempty"`
	OtdelID      *int   `json:"otdel_id" validate:"omitempty"`
}

type UserDTO struct {
	ID          int            `json:"id"`
	Fio         string         `json:"fio"`
	Email       string         `json:"email"`
	Position    string         `json:"position"`
	PhoneNumber string         `json:"phone_number"`
	Role        int            `json:"role"`
	Branch      int            `json:"branch"`
	Department  int            `json:"department"`
	Office      *int           `json:"office"`
	Otdel       *int           `json:"otdel"`
	Status      ShortStatusDTO `json:"status"`
	CreatedAt   string         `json:"created_at"`
	UpdatedAt   string         `json:"updated_at"`
}

type ShortUserDTO struct {
	ID  int    `json:"id"`
	Fio string `json:"fio"`
}
