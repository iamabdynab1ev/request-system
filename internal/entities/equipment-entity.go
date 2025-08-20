package entities

import (
	"request-system/pkg/types"
)

type Equipment struct {
	ID              uint64 `json:"id"`
	Name            string `json:"name"`
	Address         string `json:"address"`
	BranchID        uint64 `json:"branch_id"`
	OfficeID        uint64 `json:"office_id"`
	StatusID        uint64 `json:"status_id"`
	EquipmentTypeID uint64 `json:"equipment_type_id"`

	types.BaseEntity // CreatedAt, UpdatedAt

	// Поля для связанных данных (не колонки в таблице)
	Branch        *Branch        `db:"-"`
	Office        *Office        `db:"-"`
	EquipmentType *EquipmentType `db:"-"`
	Status        *Status        `db:"-"`
}
