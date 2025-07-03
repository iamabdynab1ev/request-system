package entities

import (
	"database/sql"
	"time"
)

type User struct {
	ID           uint64 `json:"id"`
	FIO          string `json:"fio"`
	Email        string `json:"email"`
	PhoneNumber  string `json:"phone_number"`
	Password     string `json:"password"`
	StatusID     int    `json:"status_id"`
	RoleID       int    `json:"role_id"`
	DepartmentID int    `json:"department_id"`

	Position sql.NullString `json:"position"`
	BranchID sql.NullInt64  `json:"branch_id"`
	OfficeID sql.NullInt64  `json:"office_id"`
	OtdelID  sql.NullInt64  `json:"otdel_id"`

	CreatedAt time.Time    `json:"created_at"`
	UpdatedAt time.Time    `json:"updated_at"`
	DeletedAt sql.NullTime `json:"deleted_at"`

	StatusName     sql.NullString `json:"status_name"`
	RoleName       sql.NullString `json:"role_name"`
	DepartmentName sql.NullString `json:"department_name"`
}
