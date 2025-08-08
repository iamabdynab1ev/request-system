package entities

import "request-system/pkg/types"

type User struct {
	ID          uint64 `json:"id"`
	Fio         string `json:"fio"`
	Email       string `json:"email"`
	PhoneNumber string `json:"phone_number"`
	Password    string `json:"-"`
	Position    string `json:"position"`

	StatusID     uint64  `json:"status_id"`
	RoleID       uint64  `json:"role_id"`
	RoleName     string  `json:"role_name"`
	BranchID     uint64  `json:"branch_id"`
	DepartmentID uint64  `json:"department_id"`
	OfficeID     *uint64 `json:"office_id"`
	OtdelID      *uint64 `json:"otdel_id"`
	PhotoURL     *string `json:"photo_url,omitempty"`

	types.BaseEntity
	types.SoftDelete
}
