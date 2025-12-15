package entities

import "time"

// Order — структура для заявки
// ВАЖНО: Добавлены теги `json`, чтобы SmartUpdate мог сопоставить map и структуру
type Order struct {
	ID              uint64     `db:"id" json:"id"`
	Name            string     `db:"name" json:"name"`
	DepartmentID    *uint64    `db:"department_id" json:"department_id"`
	StatusID        uint64     `db:"status_id" json:"status_id"`
	CreatorID       uint64     `db:"user_id" json:"user_id"` // UserID создателя часто мапится как creator_id в DTO, но в базе user_id
	OrderTypeID     *uint64    `db:"order_type_id" json:"order_type_id"`
	OtdelID         *uint64    `db:"otdel_id" json:"otdel_id"`
	PriorityID      *uint64    `db:"priority_id" json:"priority_id"`
	BranchID        *uint64    `db:"branch_id" json:"branch_id"`
	OfficeID        *uint64    `db:"office_id" json:"office_id"`
	EquipmentID     *uint64    `db:"equipment_id" json:"equipment_id"`
	EquipmentTypeID *uint64    `db:"equipment_type_id" json:"equipment_type_id"`
	ExecutorID      *uint64    `db:"executor_id" json:"executor_id"`
	Duration        *time.Time `db:"duration" json:"duration"`
	Address         *string    `db:"address" json:"address"`
	CreatedAt       time.Time  `db:"created_at" json:"created_at"`
	UpdatedAt       time.Time  `db:"updated_at" json:"updated_at"`
	DeletedAt       *time.Time `db:"deleted_at" json:"-"`
	CompletedAt     *time.Time `db:"completed_at" json:"completed_at"`

	// Метрики
	FirstResponseTimeSeconds *uint64 `db:"first_response_time_seconds" json:"first_response_time_seconds"`
	ResolutionTimeSeconds    *uint64 `db:"resolution_time_seconds" json:"resolution_time_seconds"`
	IsFirstContactResolution *bool   `db:"is_first_contact_resolution" json:"is_first_contact_resolution"`

	// Поля для Join (Read Only) - их не обновляем через SmartUpdate, тег json можно не ставить или ставить для выдачи
	CreatorName  string  `db:"creator_name" json:"creator_name,omitempty"`
	ExecutorName *string `db:"executor_name" json:"executor_name,omitempty"`
}
