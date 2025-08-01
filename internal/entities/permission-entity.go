package entities

import (
	"request-system/pkg/types"
)

type Permission struct {
	ID          uint64 `json:"id"`
	Name        string `json:"name"`
	Description string `json:"description"`

	types.BaseEntity
}
