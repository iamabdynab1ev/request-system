package entities

import (
	"request-system/pkg/types"
)

type Department struct {
	ID     int    `json:"id"`
	Name   string `json:"name"`
	Status Status `json:"status"`

	types.BaseEntity 
}
