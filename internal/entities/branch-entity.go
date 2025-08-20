package entities

import (
	"database/sql"
	"time"
)

type Branch struct {
	ID          uint64       `db:"id"`
	Name        string       `db:"name"`
	ShortName   string       `db:"short_name"`
	Address     string       `db:"address"`
	PhoneNumber string       `db:"phone_number"`
	Email       string       `db:"email"`
	EmailIndex  string       `db:"email_index"`
	OpenDate    time.Time    `db:"open_date"`
	StatusID    uint64       `db:"status_id"`
	CreatedAt   time.Time    `db:"created_at"`
	UpdatedAt   time.Time    `db:"updated_at"`
	DeletedAt   sql.NullTime `db:"deleted_at"`

	Status *Status `db:"-"`
}
