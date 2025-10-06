package entities

import "request-system/pkg/types"

type Role struct {
	ID          uint64
	Name        string
	Description string
	StatusID    uint64
	Permissions []uint64

	types.BaseEntity
	types.SoftDelete
}

