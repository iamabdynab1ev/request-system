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

const (
	dateTimeLayout = "2006-01-02 15:04:05"
)

type BranchServiceInterface interface {
	GetBranches(ctx context.Context, filter types.Filter) ([]dto.BranchListResponseDTO, uint64, error)
	FindBranch(ctx context.Context, id uint64) (*dto.BranchDTO, error)
	CreateBranch(ctx context.Context, payload dto.CreateBranchDTO) (*dto.BranchDTO, error)
	UpdateBranch(ctx context.Context, id uint64, payload dto.UpdateBranchDTO) (*dto.BranchDTO, error)
	DeleteBranch(ctx context.Context, id uint64) error
}

type BranchService struct {
	txManager        repositories.TxManagerInterface
	branchRepository repositories.BranchRepositoryInterface
	userRepository   repositories.UserRepositoryInterface
	logger           *zap.Logger
}

func NewBranchService(txManager repositories.TxManagerInterface, branchRepo repositories.BranchRepositoryInterface, userRepo repositories.UserRepositoryInterface, logger *zap.Logger) BranchServiceInterface {
	return &BranchService{
		txManager:        txManager,
		branchRepository: branchRepo,
		userRepository:   userRepo,
		logger:           logger,
	}
}

// Приватный хелпер для проверки прав, чтобы не дублировать код
func (s *BranchService) buildAuthzContext(ctx context.Context) (*authz.Context, error) {
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

// "Переводчик" из Entity в детальный DTO
func branchEntityToDTO(entity *entities.Branch) *dto.BranchDTO {
	if entity == nil {
		return nil
	}

	var dtoStatus *dto.ShortStatusDTO
	if entity.Status != nil {
		dtoStatus = &dto.ShortStatusDTO{
			ID:   uint64(entity.Status.ID), // Приведение типа, если ID в entities.Status - int
			Name: entity.Status.Name,
		}
	}

	dto := &dto.BranchDTO{
		ID:          entity.ID,
		Name:        entity.Name,
		ShortName:   entity.ShortName,
		Address:     utils.GetStringFromPtr(entity.Address),
		PhoneNumber: utils.GetStringFromPtr(entity.PhoneNumber),
		Email:       utils.GetStringFromPtr(entity.Email),
		EmailIndex:  utils.GetStringFromPtr(entity.EmailIndex),
		Status:      dtoStatus,
	}

	if entity.OpenDate != nil {
		dto.OpenDate = entity.OpenDate.Format(timeLayout)
	}
	if entity.CreatedAt != nil {
		dto.CreatedAt = entity.CreatedAt.Format(dateTimeLayout)
	}
	if entity.UpdatedAt != nil {
		dto.UpdatedAt = entity.UpdatedAt.Format(dateTimeLayout)
	}

	return dto
}

// Метод для получения списка с фильтрацией - С ИЗМЕНЕНИЯМИ
func (s *BranchService) GetBranches(ctx context.Context, filter types.Filter) ([]dto.BranchListResponseDTO, uint64, error) {
	authCtx, err := s.buildAuthzContext(ctx)
	if err != nil {
		return nil, 0, err
	}
	if !authz.CanDo(authz.BranchesView, *authCtx) {
		return nil, 0, apperrors.ErrForbidden
	}

	entities, total, err := s.branchRepository.GetBranches(ctx, filter)
	if err != nil {
		return nil, 0, err
	}

	dtos := make([]dto.BranchListResponseDTO, 0, len(entities))
	for _, b := range entities {
		branchDto := dto.BranchListResponseDTO{
			ID:          b.ID,
			Name:        b.Name,
			ShortName:   b.ShortName,
			Address:     utils.GetStringFromPtr(b.Address),
			PhoneNumber: utils.GetStringFromPtr(b.PhoneNumber),
			Email:       utils.GetStringFromPtr(b.Email),
			EmailIndex:  utils.GetStringFromPtr(b.EmailIndex),
			StatusID:    b.StatusID,
		}
		if b.OpenDate != nil {
			branchDto.OpenDate = b.OpenDate.Format(timeLayout)
		}
		if b.CreatedAt != nil {
			branchDto.CreatedAt = b.CreatedAt.Format(dateTimeLayout)
		}
		dtos = append(dtos, branchDto)
	}
	return dtos, total, nil
}

// Метод для поиска одной записи
func (s *BranchService) FindBranch(ctx context.Context, id uint64) (*dto.BranchDTO, error) {
	authCtx, err := s.buildAuthzContext(ctx)
	if err != nil {
		return nil, err
	}
	if !authz.CanDo(authz.BranchesView, *authCtx) {
		return nil, apperrors.ErrForbidden
	}
	entity, err := s.branchRepository.FindBranch(ctx, id)
	if err != nil {
		return nil, err
	}
	return branchEntityToDTO(entity), nil
}

// Метод для создания записи
func (s *BranchService) CreateBranch(ctx context.Context, payload dto.CreateBranchDTO) (*dto.BranchDTO, error) {
	authCtx, err := s.buildAuthzContext(ctx)
	if err != nil {
		return nil, err
	}
	if !authz.CanDo(authz.BranchesCreate, *authCtx) {
		return nil, apperrors.ErrForbidden
	}

	var openDatePtr *time.Time
	if payload.OpenDate != "" {
		parsedDate, err := time.Parse(timeLayout, payload.OpenDate)
		if err != nil {
			return nil, apperrors.NewBadRequestError(fmt.Sprintf("Неверный формат даты: %s", payload.OpenDate))
		}
		openDatePtr = &parsedDate
	}
	entity := entities.Branch{
		Name:        payload.Name,
		ShortName:   payload.ShortName,
		Address:     utils.StringToPtr(payload.Address),
		PhoneNumber: utils.StringToPtr(payload.PhoneNumber),
		Email:       utils.StringToPtr(payload.Email),
		EmailIndex:  utils.StringToPtr(payload.EmailIndex),
		OpenDate:    openDatePtr,
		StatusID:    payload.StatusID,
	}

	var newBranchID uint64

	err = s.txManager.RunInTransaction(ctx, func(tx pgx.Tx) error {
		var txErr error
		// ИСПРАВЛЕНИЕ: Вызов CreateBranch теперь полностью соответствует интерфейсу
		newBranchID, txErr = s.branchRepository.CreateBranch(ctx, tx, entity)
		return txErr
	})
	if err != nil {
		s.logger.Error("Ошибка в транзакции создания филиала", zap.Error(err))
		return nil, err
	}

	createdBranch, err := s.branchRepository.FindBranch(ctx, newBranchID)
	if err != nil {
		s.logger.Error("Не удалось найти только что созданный филиал", zap.Uint64("id", newBranchID), zap.Error(err))
		return nil, err
	}

	return branchEntityToDTO(createdBranch), nil
}

// UpdateBranch - ИСПРАВЛЕН
func (s *BranchService) UpdateBranch(ctx context.Context, id uint64, payload dto.UpdateBranchDTO) (*dto.BranchDTO, error) {
	authCtx, err := s.buildAuthzContext(ctx)
	if err != nil {
		return nil, err
	}
	if !authz.CanDo(authz.BranchesUpdate, *authCtx) {
		return nil, apperrors.ErrForbidden
	}

	existingEntity, err := s.branchRepository.FindBranch(ctx, id)
	if err != nil {
		return nil, err
	}

	// Ваша логика обновления полей
	if payload.Name != nil {
		existingEntity.Name = *payload.Name
	}
	if payload.ShortName != nil {
		existingEntity.ShortName = *payload.ShortName
	}
	if payload.Address != nil {
		existingEntity.Address = payload.Address
	}
	if payload.PhoneNumber != nil {
		existingEntity.PhoneNumber = payload.PhoneNumber
	}
	if payload.Email != nil {
		existingEntity.Email = payload.Email
	}
	if payload.EmailIndex != nil {
		existingEntity.EmailIndex = payload.EmailIndex
	}
	if payload.StatusID != nil {
		existingEntity.StatusID = *payload.StatusID
	}
	if payload.OpenDate != nil {
		openDate, err := time.Parse(timeLayout, *payload.OpenDate)
		if err != nil {
			return nil, apperrors.NewBadRequestError(fmt.Sprintf("Неверный формат даты: %s", *payload.OpenDate))
		}
		existingEntity.OpenDate = &openDate
	}
	err = s.txManager.RunInTransaction(ctx, func(tx pgx.Tx) error {
		// ИСПРАВЛЕНИЕ: UpdateBranch теперь возвращает только ошибку
		return s.branchRepository.UpdateBranch(ctx, tx, id, *existingEntity)
	})
	if err != nil {
		s.logger.Error("Ошибка в транзакции обновления филиала", zap.Error(err))
		return nil, err
	}

	updatedEntity, err := s.branchRepository.FindBranch(ctx, id)
	if err != nil {
		return nil, err
	}
	return branchEntityToDTO(updatedEntity), nil
}

// Метод для удаления записи
func (s *BranchService) DeleteBranch(ctx context.Context, id uint64) error {
	authCtx, err := s.buildAuthzContext(ctx)
	if err != nil {
		return err
	}
	if !authz.CanDo(authz.BranchesDelete, *authCtx) {
		return apperrors.ErrForbidden
	}
	return s.branchRepository.DeleteBranch(ctx, id)
}
