package entities

import (
	"request-system/pkg/types"
)

type Priority struct {
	ID        uint64 `json:"id"`
	Icon      string `json:"icon"`
	Name      string `json:"name"`
	Rate      int    `json:"rate"`
	CreatedAt string `json:"created_at"`
	UpdatedAt string `json:"updated_at"`
	Code      string `json:"code"`
	types.BaseEntity
}
