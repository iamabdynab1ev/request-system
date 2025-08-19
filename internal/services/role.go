package services

import (
	"context"
	"fmt"
	"log"
	"request-system/internal/authz"
	"request-system/internal/dto"
	"request-system/internal/repositories"
	apperrors "request-system/pkg/errors"
	"request-system/pkg/utils"

	"github.com/jackc/pgx/v5"
	"go.uber.org/zap"
)

type RoleServiceInterface interface {
	GetRoles(ctx context.Context, limit, offset uint64) (*dto.PaginatedResponse[dto.RoleDTO], error)
	FindRole(ctx context.Context, id uint64) (*dto.RoleDTO, error)
	CreateRole(ctx context.Context, dto dto.CreateRoleDTO) (*dto.RoleDTO, error)
	UpdateRole(ctx context.Context, id uint64, dto dto.UpdateRoleDTO) (*dto.RoleDTO, error)
	DeleteRole(ctx context.Context, id uint64) error
}

type RoleService struct {
	repo                  repositories.RoleRepositoryInterface
	userRepo              repositories.UserRepositoryInterface
	authPermissionService AuthPermissionServiceInterface // <-- ВОТ ГДЕ ОШИБКА
	logger                *zap.Logger
}

func NewRoleService(
	repo repositories.RoleRepositoryInterface,
	userRepo repositories.UserRepositoryInterface,
	authPermissionService AuthPermissionServiceInterface,
	logger *zap.Logger,
) RoleServiceInterface {
	return &RoleService{repo: repo, userRepo: userRepo, authPermissionService: authPermissionService, logger: logger}
}

// buildAuthzContext - приватный хелпер для сборки контекста авторизации
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
	return &authz.Context{Actor: actor, Permissions: permissionsMap, Target: nil}, nil
}

func (s *RoleService) GetRoles(ctx context.Context, limit uint64, offset uint64) (*dto.PaginatedResponse[dto.RoleDTO], error) {
	authContext, err := s.buildAuthzContext(ctx)
	if err != nil {
		return nil, err
	}
	if !authz.CanDo(authz.RolesView, *authContext) {
		return nil, apperrors.ErrForbidden
	}

	roles, total, err := s.repo.GetRoles(ctx, limit, offset)
	if err != nil {
		return nil, err
	}

	var currentPage uint64 = 1
	if limit > 0 {
		currentPage = (offset / limit) + 1
	}

	return &dto.PaginatedResponse[dto.RoleDTO]{
		List:       roles,
		Pagination: dto.PaginationObject{TotalCount: total, Page: currentPage, Limit: limit},
	}, nil
}

func (s *RoleService) FindRole(ctx context.Context, id uint64) (*dto.RoleDTO, error) {
	authContext, err := s.buildAuthzContext(ctx)
	if err != nil {
		return nil, err
	}

	if !authz.CanDo(authz.RolesView, *authContext) {
		return nil, apperrors.ErrForbidden
	}
	return s.repo.FindByID(ctx, id)
}

func (s *RoleService) CreateRole(ctx context.Context, dto dto.CreateRoleDTO) (*dto.RoleDTO, error) {
	authContext, err := s.buildAuthzContext(ctx)
	if err != nil {
		return nil, err
	}

	if !authz.CanDo(authz.RolesCreate, *authContext) {
		return nil, apperrors.ErrForbidden
	}

	tx, err := s.repo.BeginTx(ctx)
	if err != nil {
		return nil, fmt.Errorf("ошибка создания роли: %w", err)
	}
	defer func() {
		if err := tx.Rollback(ctx); err != nil && err != pgx.ErrTxClosed {
			log.Println("Ошибка при rollback:", err)
		}
	}()

	newRoleID, err := s.repo.CreateRoleInTx(ctx, tx, dto)
	if err != nil {
		return nil, fmt.Errorf("ошибка создания роли: %w", err)
	}

	if len(dto.PermissionIDs) > 0 {
		if err := s.repo.LinkPermissionsToRoleInTx(ctx, tx, newRoleID, dto.PermissionIDs); err != nil {
			return nil, fmt.Errorf("ошибка привязки прав: %w", err)
		}
	}
	if err := tx.Commit(ctx); err != nil {
		return nil, fmt.Errorf("ошибка создания роли: %w", err)
	}
	s.authPermissionService.InvalidateRolePermissionsCache(ctx, newRoleID)
	return s.repo.FindByID(ctx, newRoleID)
}

func (s *RoleService) UpdateRole(ctx context.Context, id uint64, dto dto.UpdateRoleDTO) (*dto.RoleDTO, error) {
	authContext, err := s.buildAuthzContext(ctx)
	if err != nil {
		return nil, err
	}

	if !authz.CanDo(authz.RolesUpdate, *authContext) {
		return nil, apperrors.ErrForbidden
	}

	tx, err := s.repo.BeginTx(ctx)
	if err != nil {
		return nil, fmt.Errorf("ошибка обновления роли: %w", err)
	}
	defer func() {
		if err := tx.Rollback(ctx); err != nil && err != pgx.ErrTxClosed {
			log.Println("Ошибка при rollback:", err)
		}
	}()

	if err := s.repo.UpdateRoleInTx(ctx, tx, id, dto); err != nil {
		return nil, fmt.Errorf("ошибка обновления роли: %w", err)
	}

	if dto.PermissionIDs != nil {
		if err := s.repo.UnlinkAllPermissionsFromRoleInTx(ctx, tx, id); err != nil {
			return nil, fmt.Errorf("ошибка отвязки прав: %w", err)
		}
		if err := s.repo.LinkPermissionsToRoleInTx(ctx, tx, id, *dto.PermissionIDs); err != nil {
			return nil, fmt.Errorf("ошибка привязки прав: %w", err)
		}
	}

	if err := tx.Commit(ctx); err != nil {
		return nil, fmt.Errorf("ошибка обновления роли: %w", err)
	}
	s.authPermissionService.InvalidateRolePermissionsCache(ctx, id)
	return s.repo.FindByID(ctx, id)
}

func (s *RoleService) DeleteRole(ctx context.Context, id uint64) error {
	authContext, err := s.buildAuthzContext(ctx)
	if err != nil {
		return err
	}
	if !authz.CanDo(authz.RolesDelete, *authContext) {
		return apperrors.ErrForbidden
	}
	s.authPermissionService.InvalidateRolePermissionsCache(ctx, id)
	return s.repo.DeleteRole(ctx, id)
}
