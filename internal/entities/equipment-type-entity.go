package entities

import (
	"request-system/pkg/types"
)
type EquipmentType struct {
	ID    int        `json:"id"`
	Name  string     `json:"name"`

	types.BaseEntity
}
