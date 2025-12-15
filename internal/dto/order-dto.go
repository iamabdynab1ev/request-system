package dto

import (
	"time"
)

type OrderResponseDTO struct {
	ID              uint64                  `json:"id"`
	Name            string                  `json:"name"`
	OrderTypeID     *uint64                 `json:"order_type_id,omitempty"`
	Address         *string                 `json:"address,omitempty"`
	CreatorID       uint64                  `json:"creator_id"`
	ExecutorID      *uint64                 `json:"executor_id,omitempty"`
	DepartmentID    *uint64                 `json:"department_id"`
	OtdelID         *uint64                 `json:"otdel_id,omitempty"`
	BranchID        *uint64                 `json:"branch_id,omitempty"`
	OfficeID        *uint64                 `json:"office_id,omitempty"`
	EquipmentID     *uint64                 `json:"equipment_id,omitempty"`
	EquipmentTypeID *uint64                 `json:"equipment_type_id,omitempty"`
	StatusID        uint64                  `json:"status_id"`
	PriorityID      *uint64                 `json:"priority_id,omitempty"`
	Attachments     []AttachmentResponseDTO `json:"attachments"`
	Duration        *time.Time              `json:"duration,omitempty"`
	CreatorName     string                  `json:"creator_name"`
	ExecutorName    *string                 `json:"executor_name,omitempty"`
	CreatedAt       string                  `json:"created_at"`
	UpdatedAt       string                  `json:"updated_at"`
	CompletedAt     *time.Time              `json:"completed_at,omitempty"`

	// Метрики (показатели)
	ResolutionTimeSeconds      *uint64 `json:"resolution_time_seconds,omitempty"`
	ResolutionTimeFormatted    string  `json:"resolution_time_formatted,omitempty"`
	FirstResponseTimeSeconds   *uint64 `json:"first_response_time_seconds,omitempty"`
	FirstResponseTimeFormatted string  `json:"first_response_time_formatted,omitempty"`
}

type CreateOrderDTO struct {
	Name        string     `json:"name" validate:"required"`
	OrderTypeID *uint64    `json:"order_type_id" validate:"required"`
	Address     *string    `json:"address,omitempty"`
	Comment     *string    `json:"comment,omitempty"`
	Duration    *time.Time `json:"duration,omitempty"`

	// Орг. структура
	DepartmentID *uint64 `json:"department_id,omitempty"`
	OtdelID      *uint64 `json:"otdel_id,omitempty"`
	BranchID     *uint64 `json:"branch_id,omitempty"`
	OfficeID     *uint64 `json:"office_id,omitempty"`

	// Специфика заявки
	PriorityID      *uint64 `json:"priority_id,omitempty"`
	ExecutorID      *uint64 `json:"executor_id,omitempty"`
	EquipmentID     *uint64 `json:"equipment_id,omitempty"`
	EquipmentTypeID *uint64 `json:"equipment_type_id,omitempty"`
}

type UpdateOrderDTO struct {
	Name     *string    `json:"name,omitempty" validate:"omitempty,min=5"`
	Address  *string    `json:"address,omitempty" validate:"omitempty,min=5"`
	Comment  *string    `json:"comment,omitempty"`
	Duration *time.Time `json:"duration,omitempty"`

	DepartmentID *uint64 `json:"department_id,omitempty"`
	OtdelID      *uint64 `json:"otdel_id,omitempty"`
	BranchID     *uint64 `json:"branch_id,omitempty"`
	OfficeID     *uint64 `json:"office_id,omitempty"`

	EquipmentID     *uint64 `json:"equipment_id,omitempty"`
	EquipmentTypeID *uint64 `json:"equipment_type_id,omitempty"`
	ExecutorID      *uint64 `json:"executor_id,omitempty"`
	StatusID        *uint64 `json:"status_id,omitempty"`
	PriorityID      *uint64 `json:"priority_id,omitempty"`
}

type OrderListResponseDTO struct {
	List       []OrderResponseDTO `json:"list"`
	TotalCount uint64             `json:"total_count"`
}
