package repositories

import (
	"context"
	"request-system/internal/dto"
	"request-system/internal/entities"
)

type RoleRepositoryInterface interface {
	GetRoles(ctx context.Context, limit uint64, offset uint64) ([]entities.Role, error)
	FindRole(ctx context.Context, id uint64) (*entities.Role, error)
	CreateRole(ctx context.Context, payload dto.CreateRoleDTO) error
	UpdateRole(ctx context.Context, id uint64, payload dto.UpdateRoleDTO) error
	DeleteRole(ctx context.Context, id uint64) error
}

type RoleRepository struct {}

func NewRoleRepository() *RoleRepository {
	return &RoleRepository{}
}

func (r *RoleRepository) GetRoles(ctx context.Context, limit uint64, offset uint64) ([]entities.Role, error) {
	return []entities.Role{}, nil
}

func (r *RoleRepository) FindRole(ctx context.Context, id uint64) (*entities.Role, error) {
	return nil, nil
}

func (r *RoleRepository) CreateRole(ctx context.Context, payload dto.CreateRoleDTO) error {
	return nil
}

func (r *RoleRepository) UpdateRole(ctx context.Context, id uint64, payload dto.UpdateRoleDTO) error {
	return nil
}

func (r *RoleRepository) DeleteRole(ctx context.Context, id uint64) error {
	return nil
}
