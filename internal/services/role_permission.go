package services

import (
	"context"
	"fmt"

	"request-system/internal/dto"
	"request-system/internal/repositories"

	"go.uber.org/zap"
)

type RolePermissionServiceInterface interface {
	GetRolePermissions(ctx context.Context, limit uint64, offset uint64) ([]dto.RolePermissionDTO, uint64, error)
	FindRolePermission(ctx context.Context, id uint64) (*dto.RolePermissionDTO, error)
	CreateRolePermission(ctx context.Context, dto dto.CreateRolePermissionDTO) (*dto.RolePermissionDTO, error)
	UpdateRolePermission(ctx context.Context, id uint64, dto dto.UpdateRolePermissionDTO) (*dto.RolePermissionDTO, error)
	DeleteRolePermission(ctx context.Context, id uint64) error
}

type RolePermissionService struct {
	rpRepository          repositories.RolePermissionRepositoryInterface
	authPermissionService AuthPermissionServiceInterface
	logger                *zap.Logger
}

func NewRolePermissionService(
	rpRepository repositories.RolePermissionRepositoryInterface,
	authPermissionService AuthPermissionServiceInterface,
	logger *zap.Logger,
) RolePermissionServiceInterface {
	return &RolePermissionService{
		rpRepository:          rpRepository,
		authPermissionService: authPermissionService,
		logger:                logger,
	}
}

func (s *RolePermissionService) GetRolePermissions(ctx context.Context, limit uint64, offset uint64) ([]dto.RolePermissionDTO, uint64, error) {
	rolePermissions, total, err := s.rpRepository.GetRolePermissions(ctx, limit, offset)
	if err != nil {
		s.logger.Error("Ошибка получения списка связей роли-привилегии из репозитория", zap.Error(err))
		return nil, 0, fmt.Errorf("ошибка получения связей роли-привилегии: %w", err)
	}
	if rolePermissions == nil {
		rolePermissions = []dto.RolePermissionDTO{}
	}
	return rolePermissions, total, nil
}

func (s *RolePermissionService) FindRolePermission(ctx context.Context, id uint64) (*dto.RolePermissionDTO, error) {
	rp, err := s.rpRepository.FindRolePermission(ctx, id)
	if err != nil {
		s.logger.Error("Ошибка поиска связи роли-привилегии", zap.Uint64("id", id), zap.Error(err))
		return nil, fmt.Errorf("ошибка поиска связи роли-привилегии: %w", err)
	}
	return rp, nil
}

func (s *RolePermissionService) CreateRolePermission(ctx context.Context, dto dto.CreateRolePermissionDTO) (*dto.RolePermissionDTO, error) {
	createdRP, err := s.rpRepository.CreateRolePermission(ctx, dto)
	if err != nil {
		s.logger.Error("CreateRolePermission: ошибка при создании связи", zap.Any("dto", dto), zap.Error(err))
		return nil, err
	}
	s.logger.Info("CreateRolePermission: успешно создана связь", zap.Uint64("role_id", dto.RoleID), zap.Uint64("permission_id", dto.PermissionID))

	// НОВОЕ: Инвалидация кеша для этой роли
	if err := s.authPermissionService.InvalidateRolePermissionsCache(ctx, dto.RoleID); err != nil { // <-- ИСПРАВЛЕНО: добавлена ctx
		s.logger.Error("CreateRolePermission: ошибка инвалидации кеша привилегий для роли", zap.Uint64("role_id", dto.RoleID), zap.Error(err))
	}
	return createdRP, nil
}

func (s *RolePermissionService) UpdateRolePermission(ctx context.Context, id uint64, dto dto.UpdateRolePermissionDTO) (*dto.RolePermissionDTO, error) {
	oldRP, err := s.rpRepository.FindRolePermission(ctx, id)
	if err != nil {
		s.logger.Error("UpdateRolePermission: не удалось найти старую связь для инвалидации кеша", zap.Uint64("id", id), zap.Error(err))
		return nil, err
	}

	updatedRP, err := s.rpRepository.UpdateRolePermission(ctx, id, dto)
	if err != nil {
		s.logger.Error("UpdateRolePermission: ошибка при обновлении связи", zap.Uint64("id", id), zap.Any("dto", dto), zap.Error(err))
		return nil, err
	}
	s.logger.Info("UpdateRolePermission: успешно обновлена связь", zap.Uint64("id", id), zap.Uint64("role_id", dto.RoleID), zap.Uint64("permission_id", dto.PermissionID))

	// НОВОЕ: Инвалидация кеша. Если role_id изменился, инвалидируем кеш старой и новой роли.
	if oldRP.RoleID != dto.RoleID {
		if err := s.authPermissionService.InvalidateRolePermissionsCache(ctx, oldRP.RoleID); err != nil { // <-- ИСПРАВЛЕНО: добавлена ctx
			s.logger.Warn("UpdateRolePermission: ошибка инвалидации кеша для СТАРОЙ роли", zap.Uint64("role_id", oldRP.RoleID), zap.Error(err))
		}
	}
	if err := s.authPermissionService.InvalidateRolePermissionsCache(ctx, dto.RoleID); err != nil { // <-- ИСПРАВЛЕНО: добавлена ctx
		s.logger.Error("UpdateRolePermission: ошибка инвалидации кеша для НОВОЙ роли", zap.Uint64("role_id", dto.RoleID), zap.Error(err))
	}
	return updatedRP, nil
}

func (s *RolePermissionService) DeleteRolePermission(ctx context.Context, id uint64) error {
	rpToDelete, err := s.rpRepository.FindRolePermission(ctx, id)
	if err != nil {
		s.logger.Error("DeleteRolePermission: не удалось найти связь для инвалидации кеша", zap.Uint64("id", id), zap.Error(err))
		return err
	}

	err = s.rpRepository.DeleteRolePermission(ctx, id)
	if err != nil {
		s.logger.Error("DeleteRolePermission: ошибка при удалении связи", zap.Uint64("id", id), zap.Error(err))
		return err
	}
	s.logger.Info("DeleteRolePermission: успешно удалена связь", zap.Uint64("id", id), zap.Uint64("role_id", rpToDelete.RoleID))

	// НОВОЕ: Инвалидация кеша для этой роли после удаления связи
	if err := s.authPermissionService.InvalidateRolePermissionsCache(ctx, rpToDelete.RoleID); err != nil { 
		s.logger.Error("DeleteRolePermission: ошибка инвалидации кеша привилегий для роли", zap.Uint64("role_id", rpToDelete.RoleID), zap.Error(err))
	}
	return nil
}
