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

type OtdelServiceInterface interface {
	GetOtdels(ctx context.Context, filter types.Filter) ([]dto.OtdelDTO, uint64, error)
	FindOtdel(ctx context.Context, id uint64) (*dto.OtdelDTO, error)
	CreateOtdel(ctx context.Context, dto dto.CreateOtdelDTO) (*dto.OtdelDTO, error)
	UpdateOtdel(ctx context.Context, id uint64, dto dto.UpdateOtdelDTO) (*dto.OtdelDTO, error)
	DeleteOtdel(ctx context.Context, id uint64) error
}

type OtdelService struct {
	otdelRepository repositories.OtdelRepositoryInterface
	userRepository  repositories.UserRepositoryInterface
	logger          *zap.Logger
}

func NewOtdelService(otdelRepo repositories.OtdelRepositoryInterface, userRepo repositories.UserRepositoryInterface, logger *zap.Logger) OtdelServiceInterface {
	return &OtdelService{
		otdelRepository: otdelRepo,
		userRepository:  userRepo,
		logger:          logger,
	}
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

func otdelEntityToDTO(entity *entities.Otdel) *dto.OtdelDTO {
	if entity == nil {
		return nil
	}
	return &dto.OtdelDTO{
		ID:            entity.ID,
		Name:          entity.Name,
		StatusID:      entity.StatusID,
		DepartmentsID: entity.DepartmentsID,
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

func (s *OtdelService) CreateOtdel(ctx context.Context, dto dto.CreateOtdelDTO) (*dto.OtdelDTO, error) {
	authCtx, err := s.buildAuthzContext(ctx)
	if err != nil {
		return nil, err
	}
	if !authz.CanDo(authz.OtdelsCreate, *authCtx) {
		return nil, apperrors.ErrForbidden
	}

	now := time.Now()
	entity := entities.Otdel{
		Name:          dto.Name,
		StatusID:      dto.StatusID,
		DepartmentsID: dto.DepartmentsID,
		BaseEntity:    types.BaseEntity{CreatedAt: &now, UpdatedAt: &now},
	}
	created, err := s.otdelRepository.CreateOtdel(ctx, entity)
	if err != nil {
		return nil, err
	}
	return otdelEntityToDTO(created), nil
}

func (s *OtdelService) UpdateOtdel(ctx context.Context, id uint64, dto dto.UpdateOtdelDTO) (*dto.OtdelDTO, error) {
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

	if dto.Name != "" {
		existing.Name = dto.Name
	}
	if dto.StatusID != 0 {
		existing.StatusID = dto.StatusID
	}
	if dto.DepartmentsID != 0 {
		existing.DepartmentsID = dto.DepartmentsID
	}

	updated, err := s.otdelRepository.UpdateOtdel(ctx, id, *existing)
	if err != nil {
		return nil, err
	}
	return otdelEntityToDTO(updated), nil
}

func (s *OtdelService) DeleteOtdel(ctx context.Context, id uint64) error {
	authCtx, err := s.buildAuthzContext(ctx)
	if err != nil {
		return err
	}
	if !authz.CanDo(authz.OtdelsDelete, *authCtx) {
		return apperrors.ErrForbidden
	}
	return s.otdelRepository.DeleteOtdel(ctx, id)
}
