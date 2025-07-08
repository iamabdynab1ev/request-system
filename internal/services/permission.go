package services

import (
	"context"
	"fmt"
	"request-system/internal/dto"
	"request-system/internal/repositories"

	"go.uber.org/zap"
)

type PermissionServiceInterface interface {
	GetPermissions(ctx context.Context, limit uint64, offset uint64) (*dto.PaginatedResponse[dto.PermissionDTO], error)
	FindPermissionByID(ctx context.Context, id uint64) (*dto.PermissionDTO, error)
	CreatePermission(ctx context.Context, dto dto.CreatePermissionDTO) (*dto.PermissionDTO, error)
	UpdatePermission(ctx context.Context, id uint64, dto dto.UpdatePermissionDTO) (*dto.PermissionDTO, error)
	DeletePermission(ctx context.Context, id uint64) error
}

type PermissionService struct {
	permissionRepository repositories.PermissionRepositoryInterface
	logger               *zap.Logger
}

func NewPermissionService(permissionRepository repositories.PermissionRepositoryInterface, logger *zap.Logger) PermissionServiceInterface {
	return &PermissionService{
		permissionRepository: permissionRepository,
		logger:               logger,
	}
}

func (s *PermissionService) GetPermissions(ctx context.Context, limit uint64, offset uint64) (*dto.PaginatedResponse[dto.PermissionDTO], error) {
	permissions, total, err := s.permissionRepository.GetPermissions(ctx, limit, offset)
	if err != nil {
		s.logger.Error("Ошибка при получении списка привилегий из репозитория", zap.Error(err))
		return nil, fmt.Errorf("ошибка получения привилегий: %w", err)
	}
	if permissions == nil {
		permissions = []dto.PermissionDTO{}
	}
	var currentPage uint64 = 1
	if limit > 0 {
		currentPage = (offset / limit) + 1
	}
	response := &dto.PaginatedResponse[dto.PermissionDTO]{
		List: permissions,
		Pagination: dto.PaginationObject{
			TotalCount: total,
			Page:       currentPage,
			Limit:      limit,
		},
	}
	return response, nil
}
func (s *PermissionService) FindPermissionByID(ctx context.Context, id uint64) (*dto.PermissionDTO, error) {
	return s.permissionRepository.FindPermissionByID(ctx, id)
}

func (s *PermissionService) CreatePermission(ctx context.Context, dto dto.CreatePermissionDTO) (*dto.PermissionDTO, error) {
	permission, err := s.permissionRepository.CreatePermission(ctx, dto)
	if err != nil {
		s.logger.Error("Ошибка при создании привилегии", zap.Error(err))
		return nil, err
	}
	s.logger.Info("Привилегия успешно создана", zap.Any("permission", permission))
	return permission, nil
}

func (s *PermissionService) UpdatePermission(ctx context.Context, id uint64, dto dto.UpdatePermissionDTO) (*dto.PermissionDTO, error) {
	return s.permissionRepository.UpdatePermission(ctx, id, dto)
}

func (s *PermissionService) DeletePermission(ctx context.Context, id uint64) error {
	return s.permissionRepository.DeletePermission(ctx, id)
}
