package entities

import (
	"time"

	"request-system/pkg/types"
)

type Office struct {
	ID       uint64    `json:"id"`
	Name     string    `json:"name"`
	Address  string    `json:"address"`
	OpenDate time.Time `json:"open_date"`

	BranchID uint64 `json:"branch_id"`
	StatusID uint64 `json:"status_id"`

	types.BaseEntity
}
