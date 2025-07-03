package entities

import (
	"request-system/pkg/types"
)

type Prorety struct {
	Id   int    `json:"id"`
	Icon string `json:"icon"`
	Name string `json:"name"`
	Rate int    `json:"rate"`
	Code string `json:"code"`
	types.BaseEntity
}
