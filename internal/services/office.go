package services

import (
	"context"
	"fmt"
	"time"

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

const timeLayout = "2006-01-02"

type OfficeServiceInterface interface {
	GetOffices(ctx context.Context, filter types.Filter) ([]dto.OfficeListResponseDTO, uint64, error)
	FindOffice(ctx context.Context, id uint64) (*dto.OfficeDTO, error)
	CreateOffice(ctx context.Context, payload dto.CreateOfficeDTO) (*dto.OfficeDTO, error)
	UpdateOffice(ctx context.Context, id uint64, payload dto.UpdateOfficeDTO) (*dto.OfficeDTO, error)
	DeleteOffice(ctx context.Context, id uint64) error
}

type OfficeService struct {
	officeRepository repositories.OfficeRepositoryInterface
	userRepository   repositories.UserRepositoryInterface
	txManager        repositories.TxManagerInterface
	logger           *zap.Logger
}

func NewOfficeService(
	officeRepo repositories.OfficeRepositoryInterface,
	userRepo repositories.UserRepositoryInterface,
	txManager repositories.TxManagerInterface,
	logger *zap.Logger,
) OfficeServiceInterface {
	return &OfficeService{
		officeRepository: officeRepo,
		userRepository:   userRepo,
		txManager:        txManager,
		logger:           logger,
	}
}

func (s *OfficeService) buildAuthzContext(ctx context.Context) (*authz.Context, error) {
	userID, err := utils.GetUserIDFromCtx(ctx)
	if err != nil {
		return nil, err
	}
	permissionsMap, err := utils.GetPermissionsMapFromCtx(ctx)
	if err != nil {
		return nil, err
	}
	// У тебя userRepository.FindUserByID, а у меня может быть другой.
	// Использую тот, что есть в BranchService - FindUserByID
	actor, err := s.userRepository.FindUserByID(ctx, userID)
	if err != nil {
		return nil, apperrors.ErrUserNotFound
	}
	return &authz.Context{Actor: actor, Permissions: permissionsMap}, nil
}

func officeEntityToDetailDTO(entity *entities.Office) *dto.OfficeDTO {
	if entity == nil {
		return nil
	}

	var createdAt, updatedAt time.Time
	if entity.CreatedAt != nil {
		createdAt = *entity.CreatedAt
	}
	if entity.UpdatedAt != nil {
		updatedAt = *entity.UpdatedAt
	}

	dto := &dto.OfficeDTO{
		ID:        entity.ID,
		Name:      entity.Name,
		Address:   entity.Address,
		OpenDate:  entity.OpenDate,
		BranchID:  entity.BranchID,
		ParentID:  entity.ParentID,
		StatusID:  entity.StatusID,
		CreatedAt: createdAt,
		UpdatedAt: updatedAt,
	}

	if entity.Branch != nil {
		dto.BranchName = &entity.Branch.Name
	}

	if entity.Parent != nil {
		dto.ParentName = &entity.Parent.Name
	}
	if entity.Status != nil {
		dto.StatusName = entity.Status.Name
	}

	return dto
}

func officeEntityToListDTO(e entities.Office) dto.OfficeListResponseDTO {
	return dto.OfficeListResponseDTO{
		ID:        e.ID,
		Name:      e.Name,
		Address:   e.Address,
		OpenDate:  e.OpenDate.Format("2006-01-02"),
		BranchID:  e.BranchID,
		StatusID:  e.StatusID,
		CreatedAt: e.CreatedAt.Format("2006-01-02 15:04:05"),
	}
}

func (s *OfficeService) GetOffices(ctx context.Context, filter types.Filter) ([]dto.OfficeListResponseDTO, uint64, error) {
	authCtx, err := s.buildAuthzContext(ctx)
	if err != nil {
		return nil, 0, err
	}
	if !authz.CanDo(authz.OfficesView, *authCtx) {
		return nil, 0, apperrors.ErrForbidden
	}

	entities, total, err := s.officeRepository.GetOffices(ctx, filter)
	if err != nil {
		return nil, 0, err
	}

	dtos := make([]dto.OfficeListResponseDTO, 0, len(entities))
	for _, e := range entities {
		dtos = append(dtos, officeEntityToListDTO(e))
	}
	return dtos, total, nil
}

func (s *OfficeService) FindOffice(ctx context.Context, id uint64) (*dto.OfficeDTO, error) {
	authCtx, err := s.buildAuthzContext(ctx)
	if err != nil {
		return nil, err
	}
	if !authz.CanDo(authz.OfficesView, *authCtx) {
		return nil, apperrors.ErrForbidden
	}

	entity, err := s.officeRepository.FindOffice(ctx, id)
	if err != nil {
		return nil, err
	}
	return officeEntityToDetailDTO(entity), nil
}

func (s *OfficeService) CreateOffice(ctx context.Context, payload dto.CreateOfficeDTO) (*dto.OfficeDTO, error) {
	authCtx, err := s.buildAuthzContext(ctx)
	if err != nil {
		return nil, err
	}
	if !authz.CanDo(authz.OfficesCreate, *authCtx) {
		return nil, apperrors.ErrForbidden
	}

	openDate, err := time.Parse("2006-01-02", payload.OpenDate)
	if err != nil {
		return nil, apperrors.NewBadRequestError(fmt.Sprintf("Неверный формат даты: %s", payload.OpenDate))
	}

	entity := entities.Office{
		Name:     payload.Name,
		Address:  payload.Address,
		OpenDate: openDate,
		BranchID: payload.BranchID,
		StatusID: payload.StatusID,
		ParentID: payload.ParentID,
	}

	var newID uint64
	err = s.txManager.RunInTransaction(ctx, func(tx pgx.Tx) error {
		var txErr error
		// ИСПРАВЛЕНИЕ: Вызываем CreateOffice с pgx.Tx и entities.Office
		newID, txErr = s.officeRepository.CreateOffice(ctx, tx, entity)
		return txErr
	})
	if err != nil {
		s.logger.Error("Ошибка в транзакции создания офиса", zap.Error(err))
		return nil, err
	}

	createdEntity, err := s.officeRepository.FindOffice(ctx, newID)
	if err != nil {
		s.logger.Error("Не удалось найти только что созданный офис", zap.Uint64("id", newID), zap.Error(err))
		return nil, err
	}

	return officeEntityToDetailDTO(createdEntity), nil
}

// UpdateOffice - ИСПРАВЛЕН
func (s *OfficeService) UpdateOffice(ctx context.Context, id uint64, payload dto.UpdateOfficeDTO) (*dto.OfficeDTO, error) {
	authCtx, err := s.buildAuthzContext(ctx)
	if err != nil {
		return nil, err
	}
	if !authz.CanDo(authz.OfficesUpdate, *authCtx) {
		return nil, apperrors.ErrForbidden
	}

	// Сначала получаем текущее состояние, чтобы не затереть непереданные поля
	existing, err := s.officeRepository.FindOffice(ctx, id)
	if err != nil {
		return nil, err
	}

	// Применяем изменения из DTO
	if payload.Name != nil {
		existing.Name = *payload.Name
	}
	if payload.Address != nil {
		existing.Address = *payload.Address
	}
	if payload.BranchID != nil {
		existing.BranchID = payload.BranchID
		existing.ParentID = nil
	}
	if payload.ParentID != nil {
		if *payload.ParentID == id {
			return nil, apperrors.NewBadRequestError("Офис не может быть родителем для самого себя")
		}
		existing.ParentID = payload.ParentID
		existing.BranchID = nil
	}
	if payload.StatusID != nil {
		existing.StatusID = *payload.StatusID
	}
	if payload.OpenDate != nil {
		openDate, errDate := time.Parse("2006-01-02", *payload.OpenDate)
		if errDate != nil {
			return nil, apperrors.NewBadRequestError("Неверный формат даты")
		}
		existing.OpenDate = openDate
	}

	err = s.txManager.RunInTransaction(ctx, func(tx pgx.Tx) error {
		return s.officeRepository.UpdateOffice(ctx, tx, id, *existing)
	})
	if err != nil {
		s.logger.Error("Ошибка в транзакции обновления офиса", zap.Error(err), zap.Uint64("id", id))
		return nil, err
	}

	updatedEntity, err := s.officeRepository.FindOffice(ctx, id)
	if err != nil {
		return nil, err
	}

	return officeEntityToDetailDTO(updatedEntity), nil
}

func (s *OfficeService) DeleteOffice(ctx context.Context, id uint64) error {
	authCtx, err := s.buildAuthzContext(ctx)
	if err != nil {
		return err
	}
	if !authz.CanDo(authz.OfficesDelete, *authCtx) {
		return apperrors.ErrForbidden
	}

	if _, err := s.officeRepository.FindOffice(ctx, id); err != nil {
		return err
	}

	if err := s.officeRepository.DeleteOffice(ctx, id); err != nil {
		s.logger.Error("Ошибка при удалении офиса", zap.Error(err), zap.Uint64("id", id))
		return err
	}

	return nil
}
