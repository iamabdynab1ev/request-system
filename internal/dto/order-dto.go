package dto

import (
	"github.com/aarondl/null/v8"
)

type OrderResponseDTO struct {
	ID                         uint64                  `json:"id"`
	Name                       string                  `json:"name"`
	OrderTypeID                null.Int                `json:"order_type_id,omitempty"`
	Address                    null.String             `json:"address,omitempty"`
	Creator                    ShortUserDTO            `json:"creator"`
	Executor                   ShortUserDTO            `json:"executor"`
	DepartmentID               null.Int                `json:"department_id"`
	OtdelID                    null.Int                `json:"otdel_id,omitempty"`
	BranchID                   null.Int                `json:"branch_id,omitempty"`
	OfficeID                   null.Int                `json:"office_id,omitempty"`
	EquipmentID                null.Int                `json:"equipment_id,omitempty"`
	EquipmentTypeID            null.Int                `json:"equipment_type_id,omitempty"`
	StatusID                   uint64                  `json:"status_id"`
	PriorityID                 null.Int                `json:"priority_id,omitempty"`
	Attachments                []AttachmentResponseDTO `json:"attachments"`
	Duration                   null.Time               `json:"duration,omitempty"`
	CreatedAt                  string                  `json:"created_at"`
	UpdatedAt                  string                  `json:"updated_at"`
	CompletedAt                null.Time               `json:"completed_at,omitempty"`
	ResolutionTimeSeconds      null.Int                `json:"resolution_time_seconds,omitempty"`
	ResolutionTimeFormatted    string                  `json:"resolution_time_formatted,omitempty"`
	FirstResponseTimeSeconds   null.Int                `json:"first_response_time_seconds,omitempty"`
	FirstResponseTimeFormatted string                  `json:"first_response_time_formatted,omitempty"`
}

// CreateOrderDTO - структура для СОЗДАНИЯ заявки.
type CreateOrderDTO struct {
	Name            string      `json:"name" validate:"required"`
	OrderTypeID     null.Int    `json:"order_type_id" validate:"required"`
	Address         null.String `json:"address,omitempty"`
	Comment         null.String `json:"comment,omitempty"`
	Duration        null.Time   `json:"duration,omitempty"`
	DepartmentID    null.Int    `json:"department_id,omitempty"`
	OtdelID         null.Int    `json:"otdel_id,omitempty"`
	BranchID        null.Int    `json:"branch_id,omitempty"`
	OfficeID        null.Int    `json:"office_id,omitempty"`
	PriorityID      null.Int    `json:"priority_id,omitempty"`
	ExecutorID      null.Int    `json:"executor_id,omitempty"`
	EquipmentID     null.Int    `json:"equipment_id,omitempty"`
	EquipmentTypeID null.Int    `json:"equipment_type_id,omitempty"`
}

type UpdateOrderDTO struct {
	Name            null.String `json:"name,omitempty" validate:"omitempty,min=5"`
	Address         null.String `json:"address,omitempty" validate:"omitempty,min=5"`
	Comment         null.String `json:"comment,omitempty"`
	Duration        null.Time   `json:"duration,omitempty"`
	DepartmentID    *uint64     `json:"department_id,omitempty"`
	OtdelID         *uint64     `json:"otdel_id,omitempty"`
	BranchID        null.Int    `json:"branch_id,omitempty"`
	OfficeID        null.Int    `json:"office_id,omitempty"`
	EquipmentID     null.Int    `json:"equipment_id,omitempty"`
	EquipmentTypeID null.Int    `json:"equipment_type_id,omitempty"`
	ExecutorID      *uint64     `json:"executor_id,omitempty"`
	StatusID        *uint64     `json:"status_id,omitempty"`
	PriorityID      *uint64     `json:"priority_id,omitempty"`
}

type OrderListResponseDTO struct {
	List       []OrderResponseDTO `json:"list"`
	TotalCount uint64             `json:"total_count"`
}
