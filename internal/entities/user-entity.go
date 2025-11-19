// Файл: internal/entities/user-entity.go
package entities

import (
	"database/sql"
	"time"

	"request-system/pkg/types"
)

type User struct {
	ID                 uint64        `json:"id"`
	Fio                string        `json:"fio"`
	Email              string        `json:"email"`
	PhoneNumber        string        `json:"phone_number"`
	Password           string        `json:"-"`
	StatusID           uint64        `json:"status_id"`
	StatusCode         string        `json:"status_code"`
	BranchID           *uint64       `json:"branch_id"`
	DepartmentID       *uint64       `json:"department_id"`
	OfficeID           *uint64       `json:"office_id"`
	OtdelID            *uint64       `json:"otdel_id"`
	PositionID         *uint64       `json:"position_id"`
	PhotoURL           *string       `json:"photo_url,omitempty"`
	IsHead             *bool         `json:"is_head,omitempty"`
	MustChangePassword bool          `json:"must_change_password"`
	TelegramChatID     sql.NullInt64 `json:"telegram_chat_id,omitempty" db:"telegram_chat_id"`

	ExternalID   *string `json:"external_id,omitempty" db:"external_id"`
	SourceSystem *string `json:"source_system,omitempty" db:"source_system"`

	TelegramLinkToken       string    `db:"-" json:"-"`
	TelegramLinkTokenExpiry time.Time `db:"-" json:"-"`

	types.BaseEntity
	types.SoftDelete
}
