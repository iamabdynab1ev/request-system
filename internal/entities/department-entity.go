package entities

import (
	"database/sql"
	"time"
)

type Department struct {
	ID        uint64       `db:"id"`
	Name      string       `db:"name"`
	StatusID  uint64       `db:"status_id"`
	CreatedAt time.Time    `db:"created_at"`
	UpdatedAt time.Time    `db:"updated_at"`
	DeletedAt sql.NullTime `db:"deleted_at"`

	ExternalID   *string `db:"external_id"`
	SourceSystem *string `db:"source_system"`
	Status       *Status `db:"-"`
}
