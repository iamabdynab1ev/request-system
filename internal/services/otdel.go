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

	"github.com/jackc/pgx/v5"
	"go.uber.org/zap"
)

type OtdelServiceInterface interface {
	GetOtdels(ctx context.Context, filter types.Filter) ([]dto.OtdelDTO, uint64, error)
	FindOtdel(ctx context.Context, id uint64) (*dto.OtdelDTO, error)
	CreateOtdel(ctx context.Context, payload dto.CreateOtdelDTO) (*dto.OtdelDTO, error)
	UpdateOtdel(ctx context.Context, id uint64, payload dto.UpdateOtdelDTO) (*dto.OtdelDTO, error)
	DeleteOtdel(ctx context.Context, id uint64) error
}

type OtdelService struct {
	txManager       repositories.TxManagerInterface
	otdelRepository repositories.OtdelRepositoryInterface
	userRepository  repositories.UserRepositoryInterface
	logger          *zap.Logger
}

func NewOtdelService(
	txManager repositories.TxManagerInterface,
	otdelRepo repositories.OtdelRepositoryInterface,
	userRepo repositories.UserRepositoryInterface,
	logger *zap.Logger,
) OtdelServiceInterface {
	return &OtdelService{
		txManager:       txManager,
		otdelRepository: otdelRepo,
		userRepository:  userRepo,
		logger:          logger,
	}
}

func otdelEntityToDTO(entity *entities.Otdel) *dto.OtdelDTO {
	if entity == nil {
		return nil
	}
	return &dto.OtdelDTO{
		ID:            entity.ID,
		Name:          entity.Name,
		StatusID:      entity.StatusID,
		DepartmentsID: entity.DepartmentsID,
		BranchID:      entity.BranchID,
		ParentID:      entity.ParentID,
		CreatedAt:     entity.CreatedAt.Format("2006-01-02 15:04:05"),
		UpdatedAt:     entity.UpdatedAt.Format("2006-01-02 15:04:05"),
	}
}

func (s *OtdelService) GetOtdels(ctx context.Context, filter types.Filter) ([]dto.OtdelDTO, uint64, error) {
	authCtx, err := s.buildAuthzContext(ctx)
	if err != nil {
		return nil, 0, err
	}
	if !authz.CanDo(authz.OtdelsView, *authCtx) {
		return nil, 0, apperrors.ErrForbidden
	}

	entities, total, err := s.otdelRepository.GetOtdels(ctx, filter)
	if err != nil {
		return nil, 0, err
	}

	dtos := make([]dto.OtdelDTO, 0, len(entities))
	for _, o := range entities {
		dtos = append(dtos, *otdelEntityToDTO(&o))
	}
	return dtos, total, nil
}

func (s *OtdelService) FindOtdel(ctx context.Context, id uint64) (*dto.OtdelDTO, error) {
	authCtx, err := s.buildAuthzContext(ctx)
	if err != nil {
		return nil, err
	}
	if !authz.CanDo(authz.OtdelsView, *authCtx) {
		return nil, apperrors.ErrForbidden
	}

	entity, err := s.otdelRepository.FindOtdel(ctx, id)
	if err != nil {
		return nil, err
	}
	return otdelEntityToDTO(entity), nil
}

func (s *OtdelService) CreateOtdel(ctx context.Context, payload dto.CreateOtdelDTO) (*dto.OtdelDTO, error) {
	authCtx, err := s.buildAuthzContext(ctx)
	if err != nil {
		return nil, err
	}
	if !authz.CanDo(authz.OtdelsCreate, *authCtx) {
		return nil, apperrors.ErrForbidden
	}

	entity := entities.Otdel{
		Name:          payload.Name,
		StatusID:      payload.StatusID,
		DepartmentsID: payload.DepartmentsID,
		BranchID:      payload.BranchID,
		ParentID:      payload.ParentID,
	}

	var newOtdelID uint64

	err = s.txManager.RunInTransaction(ctx, func(tx pgx.Tx) error {
		var txErr error
		newOtdelID, txErr = s.otdelRepository.CreateOtdel(ctx, tx, entity)
		return txErr
	})
	if err != nil {
		s.logger.Error("Ошибка в транзакции создания отдела", zap.Error(err))
		return nil, err
	}

	createdOtdel, err := s.otdelRepository.FindOtdel(ctx, newOtdelID)
	if err != nil {
		return nil, err
	}

	return otdelEntityToDTO(createdOtdel), nil
}

// UpdateOtdel - ИСПРАВЛЕН
func (s *OtdelService) UpdateOtdel(ctx context.Context, id uint64, payload dto.UpdateOtdelDTO) (*dto.OtdelDTO, error) {
	authCtx, err := s.buildAuthzContext(ctx)
	if err != nil {
		return nil, err
	}
	if !authz.CanDo(authz.OtdelsUpdate, *authCtx) {
		return nil, apperrors.ErrForbidden
	}

	existing, err := s.otdelRepository.FindOtdel(ctx, id)
	if err != nil {
		return nil, err
	}

	// Обновляем простые поля
	if payload.Name != "" {
		existing.Name = payload.Name
	}
	if payload.StatusID != 0 {
		existing.StatusID = payload.StatusID
	}

	// Обновляем родительские связи с автоматическим обнулением других родителей,
	// чтобы соответствовать правилу CHECK в базе данных.
	if payload.DepartmentsID != nil {
		existing.DepartmentsID = payload.DepartmentsID
		existing.BranchID = nil
		existing.ParentID = nil
	}
	if payload.BranchID != nil {
		existing.DepartmentsID = nil
		existing.BranchID = payload.BranchID
		existing.ParentID = nil
	}
	if payload.ParentID != nil {
		if *payload.ParentID == id {
			return nil, apperrors.NewBadRequestError("Отдел не может быть родителем для самого себя")
		}
		existing.DepartmentsID = nil
		existing.BranchID = nil
		existing.ParentID = payload.ParentID
	}

	err = s.txManager.RunInTransaction(ctx, func(tx pgx.Tx) error {
		return s.otdelRepository.UpdateOtdel(ctx, tx, id, *existing)
	})
	if err != nil {
		s.logger.Error("Ошибка в транзакции обновления отдела", zap.Error(err), zap.Uint64("id", id))
		return nil, err
	}

	updatedOtdel, err := s.otdelRepository.FindOtdel(ctx, id)
	if err != nil {
		return nil, err
	}

	return otdelEntityToDTO(updatedOtdel), nil
}

func (s *OtdelService) DeleteOtdel(ctx context.Context, id uint64) error {
	// ... (логика авторизации без изменений)
	authCtx, err := s.buildAuthzContext(ctx)
	if err != nil {
		return err
	}
	if !authz.CanDo(authz.OtdelsDelete, *authCtx) {
		return apperrors.ErrForbidden
	}

	return s.otdelRepository.DeleteOtdel(ctx, id)
}

func (s *OtdelService) buildAuthzContext(ctx context.Context) (*authz.Context, error) {
	userID, _ := utils.GetUserIDFromCtx(ctx)
	permissions, _ := utils.GetPermissionsMapFromCtx(ctx)
	actor, err := s.userRepository.FindUserByID(ctx, userID)
	if err != nil {
		return nil, apperrors.ErrUserNotFound
	}
	return &authz.Context{Actor: actor, Permissions: permissions}, nil
}
				