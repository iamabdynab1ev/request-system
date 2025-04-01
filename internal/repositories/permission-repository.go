package repositories

import (
	"context"
	"request-system/internal/dto"
	"request-system/internal/entities"
)

type PermissionRepositoryInterface interface {
	GetPermissions(ctx context.Context, limit uint64, offset uint64) ([]entities.Permission, error)
	FindPermission(ctx context.Context, id uint64) (*entities.Permission, error)
	CreatePermission(ctx context.Context, payload dto.CreatePermissionDTO) error
	UpdatePermission(ctx context.Context, id uint64, payload dto.UpdatePermissionDTO) error
	DeletePermission(ctx context.Context, id uint64) error
}

type PermissionRepository struct{}

func NewPermissionRepository() *PermissionRepository {
	return &PermissionRepository{}
}

func (r *PermissionRepository) GetPermissions(ctx context.Context, limit uint64, offset uint64) ([]entities.Permission, error) {
	return []entities.Permission{}, nil
}

func (r *PermissionRepository) FindPermission(ctx context.Context, id uint64) (*entities.Permission, error) {
	return nil, nil
}

func (r *PermissionRepository) CreatePermission(ctx context.Context, payload dto.CreatePermissionDTO) error {
	return nil
}

func (r *PermissionRepository) UpdatePermission(ctx context.Context, id uint64, payload dto.UpdatePermissionDTO) error {
	return nil
}

func (r *PermissionRepository) DeletePermission(ctx context.Context, id uint64) error {
	return nil
}
