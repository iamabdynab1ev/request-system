// Файл: internal/services/department.go
package services

import (
	"context"
	"errors"
	"net/http"

	"github.com/jackc/pgx/v5"
	"github.com/jackc/pgx/v5/pgconn"
	"go.uber.org/zap"

	"request-system/internal/authz"
	"request-system/internal/dto"
	"request-system/internal/entities"
	"request-system/internal/repositories"
	apperrors "request-system/pkg/errors"
	"request-system/pkg/types"
	"request-system/pkg/utils"
)

type DepartmentServiceInterface interface {
	GetDepartments(ctx context.Context, filter types.Filter) ([]dto.DepartmentDTO, uint64, error)
	GetDepartmentStats(ctx context.Context, filter types.Filter) ([]dto.DepartmentStatsDTO, uint64, error)
	FindDepartment(ctx context.Context, id uint64) (*dto.DepartmentDTO, error)
	CreateDepartment(ctx context.Context, payload dto.CreateDepartmentDTO) (*dto.DepartmentDTO, error)
	UpdateDepartment(ctx context.Context, id uint64, payload dto.UpdateDepartmentDTO) (*dto.DepartmentDTO, error)
	DeleteDepartment(ctx context.Context, id uint64) error
}

type DepartmentService struct {
	txManager            repositories.TxManagerInterface
	departmentRepository repositories.DepartmentRepositoryInterface
	userRepository       repositories.UserRepositoryInterface
	logger               *zap.Logger
}

func NewDepartmentService(
	txManager repositories.TxManagerInterface,
	depRepo repositories.DepartmentRepositoryInterface,
	userRepo repositories.UserRepositoryInterface,
	logger *zap.Logger,
) DepartmentServiceInterface {
	return &DepartmentService{
		txManager:            txManager,
		departmentRepository: depRepo,
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
	return &authz.Context{Actor: actor, Permissions: permissionsMap}, nil
}

func departmentEntityToDTO(entity *entities.Department) *dto.DepartmentDTO {
	if entity == nil {
		return nil
	}
	return &dto.DepartmentDTO{
		ID:        entity.ID,
		Name:      entity.Name,
		StatusID:  entity.StatusID,
		CreatedAt: entity.CreatedAt.Format("2006-01-02 15:04:05"),
		UpdatedAt: entity.UpdatedAt.Format("2006-01-02 15:04:05"),
	}
}

func (s *DepartmentService) CreateDepartment(ctx context.Context, payload dto.CreateDepartmentDTO) (*dto.DepartmentDTO, error) {
	authCtx, err := s.buildAuthzContext(ctx)
	if err != nil {
		return nil, err
	}
	if !authz.CanDo(authz.DepartmentsCreate, *authCtx) {
		return nil, apperrors.ErrForbidden
	}

	entity := entities.Department{
		Name:     payload.Name,
		StatusID: payload.StatusID,
	}

	var newID uint64
	err = s.txManager.RunInTransaction(ctx, func(tx pgx.Tx) error {
		var txErr error
		newID, txErr = s.departmentRepository.Create(ctx, tx, entity)
		return txErr
	})
	if err != nil {
		s.logger.Error("Ошибка в транзакции создания департамента", zap.Error(err))
		return nil, err
	}

	createdEntity, err := s.departmentRepository.FindDepartment(ctx, newID)
	if err != nil {
		s.logger.Error("Не удалось найти только что созданный департамент", zap.Uint64("id", newID), zap.Error(err))
		return nil, err
	}

	return departmentEntityToDTO(createdEntity), nil
}

func (s *DepartmentService) UpdateDepartment(ctx context.Context, id uint64, payload dto.UpdateDepartmentDTO) (*dto.DepartmentDTO, error) {
	authCtx, err := s.buildAuthzContext(ctx)
	if err != nil {
		return nil, err
	}
	if !authz.CanDo(authz.DepartmentsUpdate, *authCtx) {
		return nil, apperrors.ErrForbidden
	}

	existing, err := s.departmentRepository.FindDepartment(ctx, id)
	if err != nil {
		return nil, err
	}

	if payload.Name != nil {
		existing.Name = *payload.Name
	}
	if payload.StatusID != nil {
		existing.StatusID = *payload.StatusID
	}

	err = s.txManager.RunInTransaction(ctx, func(tx pgx.Tx) error {
		return s.departmentRepository.Update(ctx, tx, id, *existing)
	})
	if err != nil {
		s.logger.Error("Ошибка в транзакции обновления департамента", zap.Error(err), zap.Uint64("id", id))
		return nil, err
	}

	updatedEntity, err := s.departmentRepository.FindDepartment(ctx, id)
	if err != nil {
		return nil, err
	}

	return departmentEntityToDTO(updatedEntity), nil
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
			return apperrors.NewHttpError(http.StatusConflict,
				"Невозможно удалить департамент, так как он используется в других частях системы.",
				nil, map[string]interface{}{"department_id": id})
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
