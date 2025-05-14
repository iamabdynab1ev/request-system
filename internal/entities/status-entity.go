package entities

import "request-system/pkg/types"


type Status struct {
	ID        int        `json:"id"`
	Icon      string     `json:"icon"`
	Name      string     `json:"name"`
	Type      int        `json:"type"`

	types.BaseEntity
}