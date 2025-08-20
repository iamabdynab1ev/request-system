package services

import (
	"context"
	"request-system/internal/authz"
	"request-system/internal/dto"
	"request-system/internal/entities"
	"request-system/internal/repositories"
	apperrors "request-system/pkg/errors"
	"request-system/pkg/types"
	"request-system/pkg/utils"
	"time"

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
	authPermissionService AuthPermissionServiceInterface
	logger                *zap.Logger
}

func NewRoleService(repo repositories.RoleRepositoryInterface, userRepo repositories.UserRepositoryInterface, authService AuthPermissionServiceInterface, logger *zap.Logger) RoleServiceInterface {
	return &RoleService{repo: repo, userRepo: userRepo, authPermissionService: authService, logger: logger}
}

func (s *RoleService) buildAuthzContext(ctx context.Context) (*authz.Context, error) {
	userID, _ := utils.GetUserIDFromCtx(ctx)
	permissionsMap, _ := utils.GetPermissionsMapFromCtx(ctx)
	actor, _ := s.userRepo.FindUserByID(ctx, userID)
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

func (s *RoleService) GetRoles(ctx context.Context, filter types.Filter) ([]dto.RoleDTO, uint64, error) {
	authCtx, err := s.buildAuthzContext(ctx)
	if err != nil {
		return nil, 0, err
	}
	if !authz.CanDo(authz.RolesView, *authCtx) {
		return nil, 0, apperrors.ErrForbidden
	}

	total, err := s.repo.CountRoles(ctx, filter)
	if err != nil {
		return nil, 0, err
	}
	if total == 0 {
		return []dto.RoleDTO{}, 0, nil
	}

	entities, err := s.repo.GetRoles(ctx, filter)
	if err != nil {
		return nil, 0, err
	}

	dtos := make([]dto.RoleDTO, 0, len(entities))
	for _, role := range entities {
		dtos = append(dtos, *roleEntityToDTO(&role, []uint64{}))
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
		StatusID:    dto.StatusID,
		BaseEntity:  types.BaseEntity{CreatedAt: &now, UpdatedAt: &now},
	}

	newRoleID, err := s.repo.CreateRoleInTx(ctx, tx, entity)
	if err != nil {
		return nil, err
	}

	if len(dto.PermissionIDs) > 0 {
		if err := s.repo.LinkPermissionsToRoleInTx(ctx, tx, newRoleID, dto.PermissionIDs); err != nil {
			return nil, err
		}
	}

	if err := tx.Commit(ctx); err != nil {
		return nil, err
	}

	s.authPermissionService.InvalidateRolePermissionsCache(ctx, newRoleID)
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
	if dto.StatusID != 0 {
		existingEntity.StatusID = dto.StatusID
	}

	if err := s.repo.UpdateRoleInTx(ctx, tx, *existingEntity); err != nil {
		return nil, err
	}

	if dto.PermissionIDs != nil {
		if err := s.repo.UnlinkAllPermissionsFromRoleInTx(ctx, tx, id); err != nil {
			return nil, err
		}
		if err := s.repo.LinkPermissionsToRoleInTx(ctx, tx, id, *dto.PermissionIDs); err != nil {
			return nil, err
		}
	}

	if err := tx.Commit(ctx); err != nil {
		return nil, err
	}

	s.authPermissionService.InvalidateRolePermissionsCache(ctx, id)
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
	s.authPermissionService.InvalidateRolePermissionsCache(ctx, id)
	return s.repo.DeleteRole(ctx, id)
}
