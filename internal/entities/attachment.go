package entities

import "time"

type Attachment struct {
	ID        uint64    `db:"id"`
	OrderID   uint64    `db:"order_id"`
	UserID    uint64    `db:"user_id"`
	FileName  string    `db:"file_name"`
	FilePath  string    `db:"file_path"`
	FileType  string    `db:"file_type"`
	FileSize  int64     `db:"file_size"`
	CreatedAt time.Time `db:"created_at"`

	Order *Order `db:"-" json:"-"` // заполняется вручную
}
