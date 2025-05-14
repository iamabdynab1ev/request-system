package services

import (
	"context"

	"request-system/internal/dto"
	"request-system/internal/repositories"
)

type RoleService struct {
	roleRepository repositories.RoleRepositoryInterface
}

func NewRoleService(roleRepository repositories.RoleRepositoryInterface) *RoleService {
	return &RoleService{
		roleRepository: roleRepository,
	}
}

func (s *RoleService) GetRoles(ctx context.Context, limit uint64, offset uint64) ([]dto.RoleDTO, error) {
	return s.roleRepository.GetRoles(ctx, 1, 10)
}

func (s *RoleService) FindRole(ctx context.Context, id uint64) (*dto.RoleDTO, error) {
	return s.roleRepository.FindRole(ctx, id)
}

func (s *RoleService) CreateRole(ctx context.Context, dto dto.CreateRoleDTO) (*dto.RoleDTO, error) {
	err := s.roleRepository.CreateRole(ctx, dto)
	if err != nil {
		return nil, err
	}

	return nil, err
}

func (s *RoleService) UpdateRole(ctx context.Context, id uint64, dto dto.UpdateRoleDTO) (*dto.RoleDTO, error) {
	err := s.roleRepository.UpdateRole(ctx, id, dto)
	if err != nil {
		return nil, err
	}

	return nil, err
}

func (s *RoleService) DeleteRole(ctx context.Context, id uint64) error {
	return s.roleRepository.DeleteRole(ctx, id)
}