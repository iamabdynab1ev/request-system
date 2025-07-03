package entities

import "request-system/pkg/types"

type Status struct {
	ID   int    `json:"id"`
	Name string `json:"name"`
	Icon string `json:"icon"`
	Type string `json:"type"`
	Code string `json:"code"`

	types.BaseEntity
}
