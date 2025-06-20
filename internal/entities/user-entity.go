package entities

import "request-system/pkg/types"

type User struct {
	ID           int    `json:"id"`
	FIO          string `json:"fio"`
	Email        string `json:"email"`
	PhoneNumber  string `json:"phone_number"`
	Password     string `json:"password"`
	Position     string `json:"position"`
	StatusID     int    `json:"status_id"`
	RoleID       int    `json:"role_id"`
	BranchID     int    `json:"branch_id"`
	DepartmentID int    `json:"department_id"`
	OfficeID     *int   `json:"office_id"`
	OtdelID      *int   `json:"otdel_id"`

	types.BaseEntity
	types.SoftDelete
}

