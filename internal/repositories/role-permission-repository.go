package repositories

import (
	"context"
	"request-system/internal/dto"
	"request-system/internal/entities"
)

type RolePermissionRepositoryInterface interface {
	GetRolePermissions(ctx context.Context, limit uint64, offset uint64) ([]entities.RolePermission, error)
	FindRolePermission(ctx context.Context, id uint64) (*entities.RolePermission, error)
	CreateRolePermission(ctx context.Context, payload dto.CreateRolePermissionDTO) error
	UpdateRolePermission(ctx context.Context, id uint64, payload dto.UpdateRolePermissionDTO) error
	DeleteRolePermission(ctx context.Context, id uint64) error
}

type RolePermissionRepository struct{}

func NewRolePermissionRepository() *RolePermissionRepository {
	return &RolePermissionRepository{}
}

func (r *RolePermissionRepository) GetRolePermissions(ctx context.Context, limit uint64, offset uint64) ([]entities.RolePermission, error) {
	return []entities.RolePermission{}, nil
}

func (r *RolePermissionRepository) FindRolePermission(ctx context.Context, id uint64) (*entities.RolePermission, error) {
	return nil, nil
}

func (r *RolePermissionRepository) CreateRolePermission(ctx context.Context, payload dto.CreateRolePermissionDTO) error {
	return nil
}

func (r *RolePermissionRepository) UpdateRolePermission(ctx context.Context, id uint64, payload dto.UpdateRolePermissionDTO) error {
	return nil
}

func (r *RolePermissionRepository) DeleteRolePermission(ctx context.Context, id uint64) error {
	return nil
}
