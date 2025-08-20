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

	// Поле для данных из JOIN'а
	Status *Status `db:"-"`
}
