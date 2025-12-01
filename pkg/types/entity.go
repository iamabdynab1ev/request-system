package types

import "time"

type BaseEntity struct {
	CreatedAt *time.Time `json:"created_at" db:"created_at"` // Добавь db tag!
	UpdatedAt *time.Time `json:"updated_at" db:"updated_at"` // Добавь db tag!
}

type SoftDelete struct {
	DeletedAt *time.Time `json:"deleted_at" db:"deleted_at"` // Добавь db tag!
}
