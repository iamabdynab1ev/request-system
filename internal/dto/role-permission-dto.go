package dto

import "time"

type CreateRolePermissionDTO struct {
	RoleID       uint64 `json:"role_id" validate:"required"`
	PermissionID uint64 `json:"permission_id" validate:"required"`
}

type UpdateRolePermissionDTO struct {
	ID           uint64 `json:"id" validate:"required"`
	RoleID       uint64 `json:"role_id" validate:"omitempty"`
	PermissionID uint64 `json:"permission_id" validate:"omitempty"`
}

type RolePermissionDTO struct {
	ID           uint64    `json:"id"`
	RoleID       uint64    `json:"role_id"`
	PermissionID uint64    `json:"permission_id"`
	CreatedAt    time.Time `json:"created_at"`
	UpdatedAt    time.Time `json:"updated_at"`
}

type ShortRolePermissionDTO struct {
	ID           uint64 `json:"id"`
	RoleID       uint64 `json:"role_id"`
	PermissionID uint64 `json:"permission_id"`
}
