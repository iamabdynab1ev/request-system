package dto

type CreateRoleDTO struct {
	Name          string   `json:"name" validate:"required,max=50"`
	Description   string   `json:"description" validate:"max=255"`
	StatusID      uint64   `json:"status_id" validate:"required,gte=1"`
	PermissionIDs []uint64 `json:"permission_ids" validate:"dive,gte=1"`
}

type UpdateRoleDTO struct {
	Name          string    `json:"name" validate:"omitempty,max=50"`
	Description   string    `json:"description" validate:"omitempty,max=255"`
	StatusID      *uint64   `json:"status_id" validate:"omitempty,gte=1"`
	PermissionIDs *[]uint64 `json:"permission_ids" validate:"omitempty,dive,gte=1"`
}

type RoleDTO struct {
	ID          uint64          `json:"id"`
	Name        string          `json:"name"`
	Description string          `json:"description"`
	StatusID    uint64          `json:"status_id"`
	Permissions []PermissionDTO `json:"permissions"`
	CreatedAt   string          `json:"created_at"`
	UpdatedAt   string          `json:"updated_at"`
}
type ShortRoleDTO struct {
	ID   uint64 `json:"id"`
	Name string `json:"name"`
}
