package services

import (
	"context"

	"request-system/internal/authz"
	"request-system/internal/dto"
	"request-system/internal/repositories"
	apperrors "request-system/pkg/errors"
	"request-system/pkg/utils"

	"go.uber.org/zap"
)

type PermissionServiceInterface interface {
	GetPermissions(ctx context.Context, limit uint64, offset uint64, search string) (*dto.PaginatedResponse[dto.PermissionDTO], error)
	FindPermissionByID(ctx context.Context, id uint64) (*dto.PermissionDTO, error)
	CreatePermission(ctx context.Context, dto dto.CreatePermissionDTO) (*dto.PermissionDTO, error)
	UpdatePermission(ctx context.Context, id uint64, dto dto.UpdatePermissionDTO) (*dto.PermissionDTO, error)
	DeletePermission(ctx context.Context, id uint64) error
}

type PermissionService struct {
	permissionRepository repositories.PermissionRepositoryInterface
	userRepo             repositories.UserRepositoryInterface
	logger               *zap.Logger
}

func NewPermissionService(
	permissionRepository repositories.PermissionRepositoryInterface,
	userRepo repositories.UserRepositoryInterface,
	logger *zap.Logger,
) PermissionServiceInterface {
	return &PermissionService{
		permissionRepository: permissionRepository,
		userRepo:             userRepo,
		logger:               logger,
	}
}

func (s *PermissionService) buildAuthzContext(ctx context.Context) (*authz.Context, error) {
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

func (s *PermissionService) GetPermissions(ctx context.Context, limit uint64, offset uint64, search string) (*dto.PaginatedResponse[dto.PermissionDTO], error) {
	authContext, err := s.buildAuthzContext(ctx)
	if err != nil {
		return nil, err
	}

	if !authz.CanDo(authz.PermissionsView, *authContext) {
		s.logger.Warn("Отказано в доступе на просмотр привилегий", zap.Uint64("actorID", authContext.Actor.ID))
		return nil, apperrors.ErrForbidden
	}

	permissions, total, err := s.permissionRepository.GetPermissions(ctx, limit, offset, search)
	if err != nil {
		return nil, err
	}

	if permissions == nil {
		permissions = []dto.PermissionDTO{}
	}
	var currentPage uint64 = 1
	if limit > 0 {
		currentPage = (offset / limit) + 1
	}

	return &dto.PaginatedResponse[dto.PermissionDTO]{
		List:       permissions,
		Pagination: dto.PaginationObject{TotalCount: total, Page: currentPage, Limit: limit},
	}, nil
}

func (s *PermissionService) FindPermissionByID(ctx context.Context, id uint64) (*dto.PermissionDTO, error) {
	authContext, err := s.buildAuthzContext(ctx)
	if err != nil {
		return nil, err
	}
	if !authz.CanDo(authz.PermissionsView, *authContext) {
		return nil, apperrors.ErrForbidden
	}
	return s.permissionRepository.FindPermissionByID(ctx, id)
}

func (s *PermissionService) CreatePermission(ctx context.Context, dto dto.CreatePermissionDTO) (*dto.PermissionDTO, error) {
	authContext, err := s.buildAuthzContext(ctx)
	if err != nil {
		return nil, err
	}
	if !authContext.Permissions[authz.Superuser] {
		return nil, apperrors.ErrForbidden
	}
	return s.permissionRepository.CreatePermission(ctx, dto)
}

func (s *PermissionService) UpdatePermission(ctx context.Context, id uint64, dto dto.UpdatePermissionDTO) (*dto.PermissionDTO, error) {
	authContext, err := s.buildAuthzContext(ctx)
	if err != nil {
		return nil, err
	}
	if !authContext.Permissions[authz.Superuser] {
		return nil, apperrors.ErrForbidden
	}
	return s.permissionRepository.UpdatePermission(ctx, id, dto)
}

func (s *PermissionService) DeletePermission(ctx context.Context, id uint64) error {
	authContext, err := s.buildAuthzContext(ctx)
	if err != nil {
		return err
	}
	if !authContext.Permissions[authz.Superuser] {
		return apperrors.ErrForbidden
	}
	return s.permissionRepository.DeletePermission(ctx, id)
}
