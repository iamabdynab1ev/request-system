package services

import (
	"context"

	"request-system/internal/dto"
	"request-system/internal/repositories"
	"go.uber.org/zap"
)

type RolePermissionService struct {
	rpRepository repositories.RolePermissionRepositoryInterface
	logger 		*zap.Logger
}

func NewRolePermissionService(rpRepository repositories.RolePermissionRepositoryInterface,
	logger *zap.Logger,
	) *RolePermissionService {
	return &RolePermissionService{
		rpRepository: rpRepository,
		logger: logger, 
	}
}

func (s *RolePermissionService) GetRolePermissions(ctx context.Context, limit uint64, offset uint64) ([]dto.RolePermissionDTO, error) {
	return s.rpRepository.GetRolePermissions(ctx, 1, 10)
}

func (s *RolePermissionService) FindRolePermission(ctx context.Context, id uint64) (*dto.RolePermissionDTO, error) {
	return s.rpRepository.FindRolePermission(ctx, id)
}

func (s *RolePermissionService) CreateRolePermission(ctx context.Context, dto dto.CreateRolePermissionDTO) (*dto.RolePermissionDTO, error) {
	err := s.rpRepository.CreateRolePermission(ctx, dto)
	if err != nil {
		s.logger.Error("Ощибка при создание: ", zap.Error(err))
		return nil, err
	}
	s.logger.Info("Успешно создано")

	return nil, err
}

func (s *RolePermissionService) UpdateRolePermission(ctx context.Context, id uint64, dto dto.UpdateRolePermissionDTO) (*dto.RolePermissionDTO, error) {
	err := s.rpRepository.UpdateRolePermission(ctx, id, dto)
	if err != nil {
		return nil, err
	}

	return nil, err
}


func (s *RolePermissionService) DeleteRolePermission(ctx context.Context, id uint64) error {
	return s.rpRepository.DeleteRolePermission(ctx, id)
}