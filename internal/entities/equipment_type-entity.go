package entities

import "request-system/pkg/types"

type EquipmentType struct {
	ID   uint64 `db:"id"`
	Name string `db:"name"`
	types.BaseEntity
}
