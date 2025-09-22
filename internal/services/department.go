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

	"go.uber.org/zap"
)

type DepartmentServiceInterface interface {
	GetDepartments(ctx context.Context, filter types.Filter) ([]dto.DepartmentDTO, uint64, error)
	GetDepartmentStats(ctx context.Context, filter types.Filter) ([]dto.DepartmentStatsDTO, uint64, error)
	FindDepartment(ctx context.Context, id uint64) (*dto.DepartmentDTO, error)
	CreateDepartment(ctx context.Context, dto dto.CreateDepartmentDTO) (*dto.DepartmentDTO, error)
	UpdateDepartment(ctx context.Context, id uint64, dto dto.UpdateDepartmentDTO) (*dto.DepartmentDTO, error)
	DeleteDepartment(ctx context.Context, id uint64) error
}

type DepartmentService struct {
	departmentRepository repositories.DepartmentRepositoryInterface
	userRepository       repositories.UserRepositoryInterface
	logger               *zap.Logger
}

func NewDepartmentService(depRepo repositories.DepartmentRepositoryInterface, userRepo repositories.UserRepositoryInterface, logger *zap.Logger) DepartmentServiceInterface {
	return &DepartmentService{departmentRepository: depRepo, userRepository: userRepo, logger: logger}
}

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

func departmentEntityToDTO(entity *entities.Department) *dto.DepartmentDTO {
	if entity == nil {
		return nil
	}
	return &dto.DepartmentDTO{
		ID:        uint64(entity.ID),
		Name:      entity.Name,
		StatusID:  uint64(entity.StatusID),
		CreatedAt: entity.CreatedAt.Format("2006-01-02 15:04:05"),
		UpdatedAt: entity.UpdatedAt.Format("2006-01-02 15:04:05"),
	}
}

func (s *DepartmentService) GetDepartments(ctx context.Context, filter types.Filter) ([]dto.DepartmentDTO, uint64, error) {
	authCtx, err := s.buildAuthzContext(ctx)
	if err != nil {
		return nil, 0, err
	}
	if !authz.CanDo(authz.DepartmentsView, *authCtx) {
		return nil, 0, apperrors.ErrForbidden
	}

	entities, total, err := s.departmentRepository.GetDepartments(ctx, filter)
	if err != nil {
		return nil, 0, err
	}

	dtos := make([]dto.DepartmentDTO, 0, len(entities))
	for _, dept := range entities {
		dtos = append(dtos, *departmentEntityToDTO(&dept))
	}
	return dtos, total, nil
}

func (s *DepartmentService) FindDepartment(ctx context.Context, id uint64) (*dto.DepartmentDTO, error) {
	authCtx, err := s.buildAuthzContext(ctx)
	if err != nil {
		return nil, err
	}
	if !authz.CanDo(authz.DepartmentsView, *authCtx) {
		return nil, apperrors.ErrForbidden
	}

	entity, err := s.departmentRepository.FindDepartment(ctx, id)
	if err != nil {
		return nil, err
	}
	return departmentEntityToDTO(entity), nil
}

func (s *DepartmentService) CreateDepartment(ctx context.Context, dto dto.CreateDepartmentDTO) (*dto.DepartmentDTO, error) {
	authCtx, err := s.buildAuthzContext(ctx)
	if err != nil {
		return nil, err
	}
	if !authz.CanDo(authz.DepartmentsCreate, *authCtx) {
		return nil, apperrors.ErrForbidden
	}

	entity := entities.Department{
		Name:     dto.Name,
		StatusID: (dto.StatusID),
	}
	createdEntity, err := s.departmentRepository.CreateDepartment(ctx, entity)
	if err != nil {
		return nil, err
	}
	return departmentEntityToDTO(createdEntity), nil
}

func (s *DepartmentService) UpdateDepartment(ctx context.Context, id uint64, dto dto.UpdateDepartmentDTO) (*dto.DepartmentDTO, error) {
	authCtx, err := s.buildAuthzContext(ctx)
	if err != nil {
		return nil, err
	}
	if !authz.CanDo(authz.DepartmentsUpdate, *authCtx) {
		return nil, apperrors.ErrForbidden
	}

	updatedEntity, err := s.departmentRepository.UpdateDepartment(ctx, id, dto)
	if err != nil {

		s.logger.Error("Ошибка в репозитории при обновлении департамента", zap.Error(err), zap.Uint64("id", id))
		return nil, err
	}

	return departmentEntityToDTO(updatedEntity), nil
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

func (s *DepartmentService) GetDepartmentStats(ctx context.Context, filter types.Filter) ([]dto.DepartmentStatsDTO, uint64, error) {
	authCtx, err := s.buildAuthzContext(ctx)
	if err != nil {
		return nil, 0, err
	}
	if !authz.CanDo(authz.DepartmentsView, *authCtx) {
		return nil, 0, apperrors.ErrForbidden
	}
	return s.departmentRepository.GetDepartmentsWithStats(ctx, filter)
}
