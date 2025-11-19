package entities

import (
	"time"

	"request-system/pkg/types"
)

type Office struct {
	ID           uint64
	Name         string
	Address      string
	OpenDate     time.Time
	BranchID     uint64
	StatusID     uint64
	ExternalID   *string
	SourceSystem *string
	Branch       *Branch
	Status       *Status

	types.BaseEntity
}
