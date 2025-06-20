package entities

import (
	"request-system/pkg/types"
)

type Position struct {
	Id   int    `json:"id"`
	Name string `json:"name"`

	types.BaseEntity
}
