package services

import (
	"context"
	"errors"
	"net/http"

	"request-system/internal/authz"
	"request-system/internal/dto"
	"request-system/internal/entities"
	"request-system/internal/repositories"
	apperrors "request-system/pkg/errors"
	"request-system/pkg/types"
	"request-system/pkg/utils"

	"github.com/jackc/pgx/v5/pgconn"
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
	otdelRepository      repositories.OtdelRepositoryInterface
	userRepository       repositories.UserRepositoryInterface
	logger               *zap.Logger
}

func NewDepartmentService(
	depRepo repositories.DepartmentRepositoryInterface,
	otdelRepo repositories.OtdelRepositoryInterface,
	userRepo repositories.UserRepositoryInterface,
	logger *zap.Logger,
) DepartmentServiceInterface {
	return &DepartmentService{
		departmentRepository: depRepo,
		otdelRepository:      otdelRepo,
		userRepository:       userRepo,
		logger:               logger,
	}
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

	err = s.departmentRepository.DeleteDepartment(ctx, id)
	if err != nil {
		var pgErr *pgconn.PgError

		if errors.As(err, &pgErr) && pgErr.Code == "23503" {
			return apperrors.NewHttpError(
				http.StatusConflict, // 409 Conflict - самый подходящий статус
				"Невозможно удалить департамент, так как он используется в других частях системы (например, в отделах или заявках).",
				nil,
				map[string]interface{}{"department_id": id},
			)
		}

		return err
	}

	return nil
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
