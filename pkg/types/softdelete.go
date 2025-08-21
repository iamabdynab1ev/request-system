package types

import "time"

type SoftDelete struct {
	DeletedAt *time.Time `json:"deleted_at"`
}
