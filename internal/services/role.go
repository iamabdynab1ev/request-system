package services

import (
	"context"
	"fmt"
	"request-system/internal/dto"
	"request-system/internal/repositories"

	"github.com/jackc/pgx/v4"
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
	authPermissionService AuthPermissionServiceInterface
	logger                *zap.Logger
}

func NewRoleService(
	repo repositories.RoleRepositoryInterface,
	authPermissionService AuthPermissionServiceInterface,
	logger *zap.Logger,
) RoleServiceInterface {
	return &RoleService{
		repo:                  repo,
		authPermissionService: authPermissionService,
		logger:                logger,
	}
}

func (s *RoleService) GetRoles(ctx context.Context, limit uint64, offset uint64) (*dto.PaginatedResponse[dto.RoleDTO], error) {
	roles, total, err := s.repo.GetRoles(ctx, limit, offset)
	if err != nil {
		s.logger.Error("ошибка при получении списка ролей в сервисе", zap.Error(err))
		return nil, err
	}

	if roles == nil {
		roles = []dto.RoleDTO{}
	}

	var currentPage uint64 = 1
	if limit > 0 {
		currentPage = (offset / limit) + 1
	}

	response := &dto.PaginatedResponse[dto.RoleDTO]{
		List: roles,
		Pagination: dto.PaginationObject{
			TotalCount: total,
			Page:       currentPage,
			Limit:      limit,
		},
	}
	return response, nil
}
func (s *RoleService) FindRole(ctx context.Context, id uint64) (*dto.RoleDTO, error) {
	return s.repo.FindByID(ctx, id)
}

func (s *RoleService) CreateRole(ctx context.Context, dto dto.CreateRoleDTO) (*dto.RoleDTO, error) {
	tx, err := s.repo.BeginTx(ctx)
	if err != nil {
		s.logger.Error("CreateRole: не удалось начать транзакцию", zap.Error(err))
		return nil, fmt.Errorf("ошибка создания роли: %w", err)
	}
	defer func() {
		if rbErr := tx.Rollback(ctx); rbErr != nil && rbErr != pgx.ErrTxClosed {
			s.logger.Error("CreateRole: ошибка при откате транзакции", zap.Error(rbErr))
		}
	}()

	newRoleID, err := s.repo.CreateRoleInTx(ctx, tx, dto)
	if err != nil {
		s.logger.Error("CreateRole: ошибка при создании роли в транзакции", zap.Any("dto", dto), zap.Error(err))
		return nil, fmt.Errorf("ошибка создания роли: %w", err)
	}

	if len(dto.PermissionIDs) > 0 {
		err = s.repo.LinkPermissionsToRoleInTx(ctx, tx, newRoleID, dto.PermissionIDs)
		if err != nil {
			s.logger.Error("CreateRole: ошибка при привязке прав к роли", zap.Uint64("roleId", newRoleID), zap.Error(err))
			return nil, fmt.Errorf("ошибка привязки прав: %w", err)
		}
	}

	if err := tx.Commit(ctx); err != nil {
		s.logger.Error("CreateRole: не удалось закоммитить транзакцию", zap.Error(err))
		return nil, fmt.Errorf("ошибка создания роли: %w", err)
	}

	if err := s.authPermissionService.InvalidateRolePermissionsCache(ctx, newRoleID); err != nil {
		s.logger.Error("CreateRole: ошибка инвалидации кеша привилегий для новой роли", zap.Uint64("roleId", newRoleID), zap.Error(err))
	}

	s.logger.Info("роль успешно создана", zap.Uint64("newRoleId", newRoleID))
	return s.repo.FindByID(ctx, newRoleID)
}

func (s *RoleService) UpdateRole(ctx context.Context, id uint64, dto dto.UpdateRoleDTO) (*dto.RoleDTO, error) {
	tx, err := s.repo.BeginTx(ctx)
	if err != nil {
		s.logger.Error("UpdateRole: не удалось начать транзакцию для обновления", zap.Error(err))
		return nil, fmt.Errorf("ошибка обновления роли: %w", err)
	}
	defer tx.Rollback(ctx)

	err = s.repo.UpdateRoleInTx(ctx, tx, id, dto)
	if err != nil {
		s.logger.Error("UpdateRole: ошибка при обновлении роли в транзакции", zap.Uint64("id", id), zap.Error(err))
		return nil, fmt.Errorf("ошибка обновления роли: %w", err)
	}

	if dto.PermissionIDs != nil {
		err = s.repo.UnlinkAllPermissionsFromRoleInTx(ctx, tx, id)
		if err != nil {
			s.logger.Error("UpdateRole: ошибка при отвязке старых прав", zap.Uint64("id", id), zap.Error(err))
			return nil, fmt.Errorf("ошибка отвязки прав: %w", err)
		}
		err = s.repo.LinkPermissionsToRoleInTx(ctx, tx, id, *dto.PermissionIDs)
		if err != nil {
			s.logger.Error("UpdateRole: ошибка при привязке новых прав", zap.Uint64("id", id), zap.Error(err))
			return nil, fmt.Errorf("ошибка привязки прав: %w", err)
		}
	}

	if err := tx.Commit(ctx); err != nil {
		s.logger.Error("UpdateRole: не удалось закоммитить транзакцию обновления", zap.Error(err))
		return nil, fmt.Errorf("ошибка обновления роли: %w", err)
	}

	// Инвалидация кеша для обновленной роли после коммита
	if err := s.authPermissionService.InvalidateRolePermissionsCache(ctx, id); err != nil { // <-- ИСПРАВЛЕНО: добавлена ctx
		s.logger.Error("UpdateRole: ошибка инвалидации кеша привилегий для обновленной роли", zap.Uint64("id", id), zap.Error(err))
	}

	s.logger.Info("роль успешно обновлена", zap.Uint64("id", id))
	return s.repo.FindByID(ctx, id)
}

func (s *RoleService) DeleteRole(ctx context.Context, id uint64) error {
	if err := s.authPermissionService.InvalidateRolePermissionsCache(ctx, id); err != nil {
		s.logger.Error("DeleteRole: ошибка инвалидации кеша привилегий для удаляемой роли", zap.Uint64("id", id), zap.Error(err))
	}
	return s.repo.DeleteRole(ctx, id)
}
