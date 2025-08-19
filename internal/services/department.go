package services

import (
	"context"
	"request-system/internal/authz"
	"request-system/internal/dto"
	"request-system/internal/repositories"
	apperrors "request-system/pkg/errors"
	"request-system/pkg/types"
	"request-system/pkg/utils"

	"go.uber.org/zap"
)

type DepartmentService struct {
	departmentRepository repositories.DepartmentRepositoryInterface
	userRepository       repositories.UserRepositoryInterface
	logger               *zap.Logger
}

func NewDepartmentService(depRepo repositories.DepartmentRepositoryInterface, userRepo repositories.UserRepositoryInterface, logger *zap.Logger) *DepartmentService {
	return &DepartmentService{
		departmentRepository: depRepo,
		userRepository:       userRepo,
		logger:               logger,
	}
}

// Приватный хелпер для создания контекста авторизации
func (s *DepartmentService) buildAuthzContext(ctx context.Context) (*authz.Context, error) {
	userID, err := utils.GetUserIDFromCtx(ctx)
	if err != nil {
		return nil, err
	}
	permissionsMap, err := utils.GetPermissionsMapFromCtx(ctx)
	if err != nil {
		return nil, err
	}
	actor, err := s.userRepository.FindUserByID(ctx, userID)
	if err != nil {
		return nil, apperrors.ErrUserNotFound
	}
	return &authz.Context{Actor: actor, Permissions: permissionsMap, Target: nil}, nil
}

func (s *DepartmentService) GetDepartmentStats(ctx context.Context, filter types.Filter) ([]dto.DepartmentStatsDTO, uint64, error) {
	authCtx, err := s.buildAuthzContext(ctx)
	if err != nil {
		return nil, 0, err
	}
	if !authz.CanDo(authz.DepartmentsView, *authCtx) {
		return nil, 0, apperrors.ErrForbidden
	}

	stats, total, err := s.departmentRepository.GetDepartmentsWithStats(ctx, filter)
	if err != nil {
		s.logger.Error("Ошибка получения статистики", zap.Error(err))
		return nil, 0, err
	}
	return stats, total, nil
}

func (s *DepartmentService) GetDepartments(ctx context.Context, filter types.Filter) ([]dto.DepartmentDTO, uint64, error) {
	authCtx, err := s.buildAuthzContext(ctx)
	if err != nil {
		return nil, 0, err
	}
	if !authz.CanDo(authz.DepartmentsView, *authCtx) {
		return nil, 0, apperrors.ErrForbidden
	}

	// Вызываем улучшенный метод репозитория
	departments, total, err := s.departmentRepository.GetDepartments(ctx, filter)
	if err != nil {
		s.logger.Error("Ошибка получения департаментов", zap.Error(err))
		return nil, 0, err
	}
	return departments, total, nil
}

// ----- Остальные методы с проверкой прав -----

func (s *DepartmentService) FindDepartment(ctx context.Context, id uint64) (*dto.DepartmentDTO, error) {
	authCtx, err := s.buildAuthzContext(ctx)
	if err != nil {
		return nil, err
	}
	if !authz.CanDo(authz.DepartmentsView, *authCtx) {
		return nil, apperrors.ErrForbidden
	}
	return s.departmentRepository.FindDepartment(ctx, id)
}

func (s *DepartmentService) CreateDepartment(ctx context.Context, dto dto.CreateDepartmentDTO) (*dto.DepartmentDTO, error) {
	authCtx, err := s.buildAuthzContext(ctx)
	if err != nil {
		return nil, err
	}
	if !authz.CanDo(authz.DepartmentsCreate, *authCtx) {
		return nil, apperrors.ErrForbidden
	}
	return s.departmentRepository.CreateDepartment(ctx, dto)
}

func (s *DepartmentService) UpdateDepartment(ctx context.Context, id uint64, dto dto.UpdateDepartmentDTO) (*dto.DepartmentDTO, error) {
	authCtx, err := s.buildAuthzContext(ctx)
	if err != nil {
		return nil, err
	}
	if !authz.CanDo(authz.DepartmentsUpdate, *authCtx) {
		return nil, apperrors.ErrForbidden
	}
	return s.departmentRepository.UpdateDepartment(ctx, id, dto)
}

func (s *DepartmentService) DeleteDepartment(ctx context.Context, id uint64) error {
	authCtx, err := s.buildAuthzContext(ctx)
	if err != nil {
		return err
	}
	if !authz.CanDo(authz.DepartmentsDelete, *authCtx) {
		return apperrors.ErrForbidden
	}
	return s.departmentRepository.DeleteDepartment(ctx, id)
}
