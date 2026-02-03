// Файл: internal/entities/user_entity.go
package entities

import (
	"database/sql"
	"time"

	"request-system/pkg/types"
)

type User struct {
	ID          uint64  `json:"id" db:"id"`
	Fio         string  `json:"fio" db:"fio"`
	Username    *string `json:"username,omitempty" db:"username"`
	Email       string  `json:"email" db:"email"`
	PhoneNumber string  `json:"phone_number" db:"phone_number"`

	Password string `json:"-" db:"password"`

	StatusID uint64 `json:"status_id" db:"status_id"`

	BranchID           *uint64 `json:"branch_id" db:"branch_id"`
	OfficeID           *uint64 `json:"office_id" db:"office_id"`
	DepartmentID       *uint64 `json:"department_id" db:"department_id"`
	OtdelID            *uint64 `json:"otdel_id" db:"otdel_id"`
	PositionID         *uint64 `json:"position_id" db:"position_id"`
	PositionIDs        []uint64 `json:"position_ids" db:"-"` 
	OtdelIDs     []uint64 `json:"otdel_ids" db:"-"`
	PositionName       *string `json:"position_name,omitempty" db:"position_name"`
	BranchName         *string `json:"branch_name,omitempty" db:"branch_name"`
	DepartmentName     *string `json:"department_name,omitempty" db:"department_name"`
	OtdelName          *string `json:"otdel_name,omitempty" db:"otdel_name"`
	OfficeName         *string `json:"office_name,omitempty" db:"office_name"`
	PhotoURL           *string `json:"photo_url,omitempty" db:"photo_url"`
	IsHead             *bool   `json:"is_head,omitempty" db:"is_head"`
	MustChangePassword bool    `json:"must_change_password" db:"must_change_password"`

	ExternalID   *string `json:"external_id,omitempty" db:"external_id"`
	SourceSystem *string `json:"source_system,omitempty" db:"source_system"`

	TelegramChatID sql.NullInt64 `json:"telegram_chat_id,omitempty" db:"telegram_chat_id"`

	TelegramLinkToken       string    `db:"-" json:"-"`
	TelegramLinkTokenExpiry time.Time `db:"-" json:"-"`

	StatusCode   string `json:"status_code" db:"status_code"`
	PositionType string `json:"position_type" db:"position_type"`

	types.BaseEntity
	types.SoftDelete
}
