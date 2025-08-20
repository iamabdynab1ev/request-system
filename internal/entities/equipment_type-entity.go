package entities

import "request-system/pkg/types"

type EquipmentType struct {
	ID   uint64 `json:"id"`
	Name string `json:"name"`

	types.BaseEntity
}
