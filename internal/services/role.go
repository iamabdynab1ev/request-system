package services

import (
	"context"
	"request-system/internal/dto"
	"request-system/internal/repositories"

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
	repo   repositories.RoleRepositoryInterface
	logger *zap.Logger
}

func NewRoleService(repo repositories.RoleRepositoryInterface, logger *zap.Logger) RoleServiceInterface {
	return &RoleService{
		repo:   repo,
		logger: logger,
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
	return s.repo.FindRoleByID(ctx, id)
}

func (s *RoleService) CreateRole(ctx context.Context, dto dto.CreateRoleDTO) (*dto.RoleDTO, error) {
	tx, err := s.repo.BeginTx(ctx)
	if err != nil {
		s.logger.Error("не удалось начать транзакцию", zap.Error(err))
		return nil, err
	}
	defer tx.Rollback(ctx)

	newRoleID, err := s.repo.CreateRoleInTx(ctx, tx, dto)
	if err != nil {
		s.logger.Error("ошибка при создании роли в транзакции", zap.Any("dto", dto), zap.Error(err))
		return nil, err
	}

	if len(dto.PermissionIDs) > 0 {
		err = s.repo.LinkPermissionsToRoleInTx(ctx, tx, newRoleID, dto.PermissionIDs)
		if err != nil {
			s.logger.Error("ошибка при привязке прав к роли", zap.Int("roleId", newRoleID), zap.Error(err))
			return nil, err
		}
	}

	if err := tx.Commit(ctx); err != nil {
		s.logger.Error("не удалось закоммитить транзакцию", zap.Error(err))
		return nil, err
	}

	s.logger.Info("роль успешно создана", zap.Int("newRoleId", newRoleID))
	return s.repo.FindRoleByID(ctx, uint64(newRoleID))
}

func (s *RoleService) UpdateRole(ctx context.Context, id uint64, dto dto.UpdateRoleDTO) (*dto.RoleDTO, error) {
	tx, err := s.repo.BeginTx(ctx)
	if err != nil {
		s.logger.Error("не удалось начать транзакцию для обновления", zap.Error(err))
		return nil, err
	}
	defer tx.Rollback(ctx)

	err = s.repo.UpdateRoleInTx(ctx, tx, id, dto)
	if err != nil {
		s.logger.Error("ошибка при обновлении роли в транзакции", zap.Uint64("id", id), zap.Error(err))
		return nil, err
	}

	if dto.PermissionIDs != nil {
		err = s.repo.UnlinkAllPermissionsFromRoleInTx(ctx, tx, id)
		if err != nil {
			s.logger.Error("ошибка при отвязке старых прав", zap.Uint64("id", id), zap.Error(err))
			return nil, err
		}
		err = s.repo.LinkPermissionsToRoleInTx(ctx, tx, int(id), *dto.PermissionIDs)
		if err != nil {
			s.logger.Error("ошибка при привязке новых прав", zap.Uint64("id", id), zap.Error(err))
			return nil, err
		}
	}

	if err := tx.Commit(ctx); err != nil {
		s.logger.Error("не удалось закоммитить транзакцию обновления", zap.Error(err))
		return nil, err
	}

	s.logger.Info("роль успешно обновлена", zap.Uint64("id", id))
	return s.repo.FindRoleByID(ctx, id)
}

func (s *RoleService) DeleteRole(ctx context.Context, id uint64) error {
	return s.repo.DeleteRole(ctx, id)
}
