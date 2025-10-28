package entities

import "request-system/pkg/types"

type User struct {
	ID                 uint64  `json:"id"`
	Fio                string  `json:"fio"`
	Email              string  `json:"email"`
	PhoneNumber        string  `json:"phone_number"`
	Password           string  `json:"-"`
	StatusID           uint64  `json:"status_id"`
	StatusCode         string  `json:"status_code"`
	BranchID           *uint64 `json:"branch_id"`
	DepartmentID       uint64  `json:"department_id"`
	OfficeID           *uint64 `json:"office_id"`
	OtdelID            *uint64 `json:"otdel_id"`
	PositionID         *uint64 `json:"position_id"`
	PhotoURL           *string `json:"photo_url,omitempty"`
	IsHead             *bool   `json:"is_head,omitempty"`
	MustChangePassword bool    `json:"must_change_password"`

	types.BaseEntity
	types.SoftDelete
}
