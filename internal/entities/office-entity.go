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
	BranchID     *uint64
	ParentID     *uint64
	StatusID     uint64
	ExternalID   *string
	SourceSystem *string
	Branch       *Branch
	Parent       *Office
	Status       *Status
	types.BaseEntity
}
