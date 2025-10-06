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
	userRepository        repositories.UserRepositoryInterface
	authPermissionService AuthPermissionServiceInterface
	logger                *zap.Logger
}

func NewRolePermissionService(
	rpRepository repositories.RolePermissionRepositoryInterface,
	userRepository repositories.UserRepositoryInterface,
	authPermissionService AuthPermissionServiceInterface,
	logger *zap.Logger,
) RolePermissionServiceInterface {
	return &RolePermissionService{
		rpRepository:          rpRepository,
		userRepository:        userRepository,
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

	s.invalidateAffectedUsersCache(ctx, dto.RoleID)

	return createdRP, nil
}

func (s *RolePermissionService) UpdateRolePermission(ctx context.Context, id uint64, dto dto.UpdateRolePermissionDTO) (*dto.RolePermissionDTO, error) {
	oldRP, err := s.rpRepository.FindRolePermission(ctx, id)
	if err != nil {
		return nil, err
	}

	updatedRP, err := s.rpRepository.UpdateRolePermission(ctx, id, dto)
	if err != nil {
		return nil, err
	}

	if oldRP.RoleID != updatedRP.RoleID {
		s.invalidateAffectedUsersCache(ctx, oldRP.RoleID)
	}
	s.invalidateAffectedUsersCache(ctx, updatedRP.RoleID)

	return updatedRP, nil
}

func (s *RolePermissionService) DeleteRolePermission(ctx context.Context, id uint64) error {
	rpToDelete, err := s.rpRepository.FindRolePermission(ctx, id)
	if err != nil {
		return err
	}

	err = s.rpRepository.DeleteRolePermission(ctx, id)
	if err != nil {
		return err
	}

	s.invalidateAffectedUsersCache(ctx, rpToDelete.RoleID)

	return nil
}

func (s *RolePermissionService) invalidateAffectedUsersCache(ctx context.Context, roleID uint64) {
	userIDs, err := s.userRepository.FindUserIDsByRoleID(ctx, roleID)
	if err != nil {
		s.logger.Error("Не удалось получить ID пользователей для инвалидации кеша", zap.Uint64("roleID", roleID), zap.Error(err))
		return
	}

	if len(userIDs) > 0 {
		s.logger.Info("Инвалидация кеша для пользователей, затронутых изменением роли", zap.Uint64("roleID", roleID), zap.Int("userCount", len(userIDs)))
		for _, userID := range userIDs {
			if err := s.authPermissionService.InvalidateUserPermissionsCache(ctx, userID); err != nil {
				s.logger.Error("Ошибка при инвалидации кеша для конкретного пользователя", zap.Uint64("userID", userID), zap.Error(err))
			}
		}
	}
}
																																																																																																												