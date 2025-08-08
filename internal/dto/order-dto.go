package dto

type CreateOrderDTO struct {
	Name         string  `json:"name" validate:"required,min=5,max=255"`
	Address      string  `json:"address" validate:"required,min=5"`
	DepartmentID uint64  `json:"department_id" validate:"required,gt=0"`
	OtdelID      *uint64 `json:"otdel_id,omitempty"`
	BranchID     *uint64 `json:"branch_id,omitempty"`
	OfficeID     *uint64 `json:"office_id,omitempty"`
	EquipmentID  *uint64 `json:"equipment_id,omitempty"`
	Comment      *string `json:"comment,omitempty" validate:"omitempty,min=3"`
}

type OrderResponseDTO struct {
	ID           uint64                  `json:"id"`
	Name         string                  `json:"name"`
	Address      string                  `json:"address"`
	Creator      ShortUserDTO            `json:"creator"`
	Executor     ShortUserDTO            `json:"executor"`
	DepartmentID uint64                  `json:"department_id"`
	StatusID     uint64                  `json:"status_id"`
	PriorityID   uint64                  `json:"priority_id"`
	Attachments  []AttachmentResponseDTO `json:"attachments"`
	Comment      *string                 `json:"comment,omitempty"`
	Duration     *string                 `json:"duration"`
	CreatedAt    string                  `json:"created_at"`
	UpdatedAt    string                  `json:"updated_at"`
}

type OrderListResponseDTO struct {
	List       []OrderResponseDTO `json:"list"`
	TotalCount uint64             `json:"total_count"`
}
type DelegateOrderDTO struct {
	ExecutorID   *uint64 `json:"executor_id" validate:"required,gt=0"`
	StatusID     *uint64 `json:"status_id" validate:"required,gt=0"`
	PriorityID   *uint64 `json:"priority_id" validate:"required,gt=0"`
	Duration     *string `json:"duration,omitempty" validate:"omitempty,duration_format"`
	Comment      *string `json:"comment" validate:"required,min=3"`
	Name         *string `json:"name,omitempty" validate:"omitempty,min=5,max=255"`
	Address      *string `json:"address,omitempty" validate:"omitempty,min=5"`
	DepartmentID *uint64 `json:"department_id,omitempty" validate:"omitempty,gt=0"`
	OtdelID      *uint64 `json:"otdel_id,omitempty"`
	BranchID     *uint64 `json:"branch_id,omitempty"`
	OfficeID     *uint64 `json:"office_id,omitempty"`
	EquipmentID  *uint64 `json:"equipment_id,omitempty"`

	HasFile bool `json:"has_file,omitempty"`
}



type UpdateOrderDTO struct {
	Name         *string `json:"name,omitempty" validate:"omitempty,min=5,max=255"`
	Address      *string `json:"address,omitempty" validate:"omitempty,min=5"`
	DepartmentID *uint64 `json:"department_id,omitempty" validate:"omitempty,gt=0"`
	OtdelID      *uint64 `json:"otdel_id,omitempty"`
	BranchID     *uint64 `json:"branch_id,omitempty"`
	OfficeID     *uint64 `json:"office_id,omitempty"`
	EquipmentID  *uint64 `json:"equipment_id,omitempty"`
	ExecutorID   *uint64 `json:"executor_id,omitempty" validate:"omitempty,gt=0"`
	StatusID     *uint64 `json:"status_id,omitempty" validate:"omitempty,gt=0"`
	PriorityID   *uint64 `json:"priority_id,omitempty" validate:"omitempty,gt=0"`
	Duration     *string `json:"duration,omitempty" validate:"omitempty,duration_format"`
	Comment      *string `json:"comment,omitempty" validate:"omitempty,min=3"`
}
