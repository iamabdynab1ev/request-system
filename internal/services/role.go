package services

import (
	"context"
	"fmt"
	"time"

	"request-system/internal/authz"
	"request-system/internal/dto"
	"request-system/internal/entities"
	"request-system/internal/repositories"
	apperrors "request-system/pkg/errors"
	"request-system/pkg/types"
	"request-system/pkg/utils"

	"go.uber.org/zap"
)

type RoleServiceInterface interface {
	GetRoles(ctx context.Context, filter types.Filter) ([]dto.RoleDTO, uint64, error)
	FindRole(ctx context.Context, id uint64) (*dto.RoleDTO, error)
	CreateRole(ctx context.Context, dto dto.CreateRoleDTO) (*dto.RoleDTO, error)
	UpdateRole(ctx context.Context, id uint64, dto dto.UpdateRoleDTO) (*dto.RoleDTO, error)
	DeleteRole(ctx context.Context, id uint64) error
}

type RoleService struct {
	repo                  repositories.RoleRepositoryInterface
	userRepo              repositories.UserRepositoryInterface
	statusRepo            repositories.StatusRepositoryInterface
	authPermissionService AuthPermissionServiceInterface
	logger                *zap.Logger
}

func NewRoleService(
	repo repositories.RoleRepositoryInterface,
	userRepo repositories.UserRepositoryInterface,
	statusRepo repositories.StatusRepositoryInterface,
	authPermissionService AuthPermissionServiceInterface,
	logger *zap.Logger,
) RoleServiceInterface {
	return &RoleService{
		repo:                  repo,
		userRepo:              userRepo,
		statusRepo:            statusRepo,
		authPermissionService: authPermissionService,
		logger:                logger,
	}
}

func (s *RoleService) GetRoles(ctx context.Context, filter types.Filter) ([]dto.RoleDTO, uint64, error) {
	authCtx, err := s.buildAuthzContext(ctx)
	if err != nil {
		return nil, 0, err
	}
	if !authz.CanDo(authz.RolesView, *authCtx) {
		return nil, 0, apperrors.ErrForbidden
	}

	entities, total, err := s.repo.GetRoles(ctx, filter)
	if err != nil {
		return nil, 0, err
	}

	if len(entities) == 0 {
		return []dto.RoleDTO{}, 0, nil
	}

	dtos := make([]dto.RoleDTO, 0, len(entities))
	for _, role := range entities {
		dtos = append(dtos, dto.RoleDTO{
			ID:          role.ID,
			Name:        role.Name,
			Description: role.Description,
			StatusID:    role.StatusID,
			Permissions: role.Permissions,
			CreatedAt:   *role.CreatedAt,
			UpdatedAt:   *role.UpdatedAt,
		})
	}

	return dtos, total, nil
}

func (s *RoleService) FindRole(ctx context.Context, id uint64) (*dto.RoleDTO, error) {
	authCtx, err := s.buildAuthzContext(ctx)
	if err != nil {
		return nil, err
	}
	if !authz.CanDo(authz.RolesView, *authCtx) {
		return nil, apperrors.ErrForbidden
	}

	entity, permissions, err := s.repo.FindRoleByID(ctx, id)
	if err != nil {
		return nil, err
	}
	return roleEntityToDTO(entity, permissions), nil
}

func (s *RoleService) CreateRole(ctx context.Context, dto dto.CreateRoleDTO) (*dto.RoleDTO, error) {
	authCtx, err := s.buildAuthzContext(ctx)
	if err != nil {
		return nil, err
	}
	if !authz.CanDo(authz.RolesCreate, *authCtx) {
		return nil, apperrors.ErrForbidden
	}

	tx, err := s.repo.BeginTx(ctx)
	if err != nil {
		return nil, err
	}
	defer tx.Rollback(ctx)

	now := time.Now()
	entity := entities.Role{
		Name:        dto.Name,
		Description: dto.Description,
		BaseEntity:  types.BaseEntity{CreatedAt: &now, UpdatedAt: &now},
	}

	if dto.StatusID != nil {
		entity.StatusID = *dto.StatusID
	} else {
		defaultStatus, err := s.statusRepo.FindByCode(ctx, "ACTIVE")
		if err != nil {
			return nil, fmt.Errorf("не удалось найти статус ACTIVE: %w", err)
		}
		entity.StatusID = uint64(defaultStatus.ID)
	}

	newRoleID, err := s.repo.CreateRoleInTx(ctx, tx, entity)
	if err != nil {
		return nil, err
	}

	if len(dto.PermissionIDs) > 0 {
		if err = s.repo.LinkPermissionsToRoleInTx(ctx, tx, newRoleID, dto.PermissionIDs); err != nil {
			return nil, err
		}
	}

	if err = tx.Commit(ctx); err != nil {
		return nil, err
	}

	return s.FindRole(ctx, newRoleID)
}

func (s *RoleService) UpdateRole(ctx context.Context, id uint64, dto dto.UpdateRoleDTO) (*dto.RoleDTO, error) {
	authCtx, err := s.buildAuthzContext(ctx)
	if err != nil {
		return nil, err
	}
	if !authz.CanDo(authz.RolesUpdate, *authCtx) {
		return nil, apperrors.ErrForbidden
	}

	// Инвалидируем кеш ДО всех изменений
	s.invalidateAffectedUsersCache(ctx, id)

	tx, err := s.repo.BeginTx(ctx)
	if err != nil {
		return nil, err
	}
	defer tx.Rollback(ctx)

	existingEntity, _, err := s.repo.FindRoleByID(ctx, id)
	if err != nil {
		return nil, err
	}

	if dto.Name != "" {
		existingEntity.Name = dto.Name
	}
	if dto.Description != nil {
		existingEntity.Description = *dto.Description
	}
	if dto.StatusID != nil {
		existingEntity.StatusID = *dto.StatusID
	}
	if err := s.repo.UpdateRoleInTx(ctx, tx, *existingEntity); err != nil {
		return nil, err
	}

	if dto.PermissionIDs != nil {
		if err := s.repo.UnlinkAllPermissionsFromRoleInTx(ctx, tx, id); err != nil {
			return nil, err
		}
		if len(*dto.PermissionIDs) > 0 {
			if err := s.repo.LinkPermissionsToRoleInTx(ctx, tx, id, *dto.PermissionIDs); err != nil {
				return nil, err
			}
		}
	}

	if err := tx.Commit(ctx); err != nil {
		return nil, err
	}

	return s.FindRole(ctx, id)
}

func (s *RoleService) DeleteRole(ctx context.Context, id uint64) error {
	authCtx, err := s.buildAuthzContext(ctx)
	if err != nil {
		return err
	}
	if !authz.CanDo(authz.RolesDelete, *authCtx) {
		return apperrors.ErrForbidden
	}

	// Сначала инвалидируем кеш всех, у кого была эта роль...
	s.invalidateAffectedUsersCache(ctx, id)

	// ...а потом удаляем саму роль
	return s.repo.DeleteRole(ctx, id)
}

func (s *RoleService) invalidateAffectedUsersCache(ctx context.Context, roleID uint64) {
	userIDs, err := s.userRepo.FindUserIDsByRoleID(ctx, roleID)
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

func (s *RoleService) buildAuthzContext(ctx context.Context) (*authz.Context, error) {
	userID, err := utils.GetUserIDFromCtx(ctx)
	if err != nil {
		return nil, err
	}
	permissionsMap, err := utils.GetPermissionsMapFromCtx(ctx)
	if err != nil {
		return nil, err
	}
	actor, err := s.userRepo.FindUserByID(ctx, userID)
	if err != nil {
		return nil, apperrors.ErrUserNotFound
	}

	return &authz.Context{Actor: actor, Permissions: permissionsMap}, nil
}

func roleEntityToDTO(entity *entities.Role, permissions []uint64) *dto.RoleDTO {
	if entity == nil {
		return nil
	}
	return &dto.RoleDTO{
		ID:          entity.ID,
		Name:        entity.Name,
		Description: entity.Description,
		StatusID:    entity.StatusID,
		Permissions: permissions,
		CreatedAt:   *entity.CreatedAt,
		UpdatedAt:   *entity.UpdatedAt,
	}
}
