// dto/permission-dto.go
// Package dto содержит структуры передачи данных.
package dto

import (
	"time"
)

type PermissionDTO struct {
	ID          uint64    `json:"id"`
	Name        string    `json:"name"`
	Description string    `json:"description"`
	CreatedAt   time.Time `json:"created_at"`
	UpdatedAt   time.Time `json:"updated_at"`
}

type CreatePermissionDTO struct {
	Name        string `json:"name" validate:"required,max=100"`
	Description string `json:"description" validate:"required"`
}

type UpdatePermissionDTO struct {
	Name        string `json:"name" validate:"omitempty,max=100"`
	Description string `json:"description" validate:"omitempty"`
}

type PermissionListResponseDTO struct {
	List       []PermissionDTO `json:"list"`
	TotalCount int64           `json:"total_count"`
}

type PermissionSource struct {
	PermissionID uint64
	Source       string
}
type UIPermissionDetailDTO struct {
	ID          uint64 `json:"id" db:"id"`
	Name        string `json:"name" db:"name"`
	Description string `json:"description" db:"description"`
	Source      string `json:"source,omitempty" db:"source"`
	Status      string `json:"status,omitempty" db:"status"`
}

// Это финальная структура, которую вернет наш новый эндпоинт
type UIPermissionsResponseDTO struct {
	HasAccess []UIPermissionDetailDTO `json:"has_access"`
	NoAccess  []UIPermissionDetailDTO `json:"no_access"`
}
