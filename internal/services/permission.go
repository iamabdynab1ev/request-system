package services

import (
	"context"

	"request-system/internal/dto"
	"request-system/internal/repositories"
	"go.uber.org/zap"
)

type PermissionService struct {
	permissionRepository repositories.PermissionRepositoryInterface
	logger               *zap.Logger
}

func NewPermissionService(permissionRepository repositories.PermissionRepositoryInterface,
	logger *zap.Logger,
	) *PermissionService {
	return &PermissionService{
		permissionRepository: permissionRepository,
		logger: logger, 
	}
}

func (s *PermissionService) GetPermissions(ctx context.Context, limit uint64, offset uint64) ([]dto.PermissionDTO, error) {
	return s.permissionRepository.GetPermissions(ctx, 1, 10)
}

func (s *PermissionService) FindPermission(ctx context.Context, id uint64) (*dto.PermissionDTO, error) {
	return s.permissionRepository.FindPermission(ctx, id)
}

func (s *PermissionService) CreatePermission(ctx context.Context, dto dto.CreatePermissionDTO) (*dto.PermissionDTO, error) {
	err := s.permissionRepository.CreatePermission(ctx, dto)
	if err != nil {
		s.logger.Error("Ощибка при создание: ", zap.Error(err))
		return nil, err
	}
s.logger.Info("Успешно создан", zap.Any("payload:", dto))
	return nil, err
}

func (s *PermissionService) UpdatePermission(ctx context.Context, id uint64, dto dto.UpdatePermissionDTO) (*dto.PermissionDTO, error) {
	err := s.permissionRepository.UpdatePermission(ctx, id, dto)
	if err != nil {
		return nil, err
	}

	return nil, err
}

func (s *PermissionService) DeletePermission(ctx context.Context, id uint64) error {
	return s.permissionRepository.DeletePermission(ctx, id)
}