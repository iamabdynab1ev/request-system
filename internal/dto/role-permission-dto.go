package dto

type CreateRolePermissionDTO struct {
	RoleID       int `json:"role_id" validate:"required"`
	PermissionID int `json:"permission_id" validate:"required"`
}

type UpdateRolePermissionDTO struct {
	ID           int `json:"id" validate:"required"`
	RoleID       int `json:"role_id" validate:"omitempty"`
	PermissionID int `json:"permission_id" validate:"omitempty"`
}

type RolePermissionDTO struct {
	ID           int `json:"id"`
	RoleID       int `json:"role_id"`
	PermissionID int `json:"permission_id"`
	CreatedAt    string `json:"created_at"`
}	

type ShortRolePermissionDTO struct {
	ID           int `json:"id"`
	RoleID       int `json:"role_id"`
	PermissionID int `json:"permission_id"`
}

