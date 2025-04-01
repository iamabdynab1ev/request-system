package entities

import (
	"request-system/pkg/types"
)

type OrderDocuments struct {
	ID        int       `json:"id"`
	Name      string    `json:"name"`
	Path      string    `json:"path"`
	Type      string    `json:"type"`
	OrderID   int       `json:"order_id"`

	types.BaseEntity
}
