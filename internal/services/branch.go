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

	"go.uber.org/zap"
)

const (
	timeLayout     = "2006-01-02"
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
	branchRepository repositories.BranchRepositoryInterface
	userRepository   repositories.UserRepositoryInterface
	logger           *zap.Logger
}

func NewBranchService(branchRepo repositories.BranchRepositoryInterface, userRepo repositories.UserRepositoryInterface, logger *zap.Logger) BranchServiceInterface {
	return &BranchService{branchRepository: branchRepo, userRepository: userRepo, logger: logger}
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

	return &dto.BranchDTO{
		ID:          entity.ID,
		Name:        entity.Name,
		ShortName:   entity.ShortName,
		Address:     entity.Address,
		PhoneNumber: entity.PhoneNumber,
		Email:       entity.Email,
		EmailIndex:  entity.EmailIndex,
		OpenDate:    entity.OpenDate.Format(timeLayout),
		Status:      dtoStatus,
		CreatedAt:   entity.CreatedAt.Format(dateTimeLayout),
		UpdatedAt:   entity.UpdatedAt.Format(dateTimeLayout),
	}
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
		dtos = append(dtos, dto.BranchListResponseDTO{
			ID:          b.ID,
			Name:        b.Name,
			ShortName:   b.ShortName,
			Address:     b.Address,
			PhoneNumber: b.PhoneNumber,
			Email:       b.Email,
			EmailIndex:  b.EmailIndex,
			OpenDate:    b.OpenDate.Format(timeLayout),
			StatusID:    b.StatusID,
			CreatedAt:   b.CreatedAt.Format(dateTimeLayout),
		})
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

	openDate, err := time.Parse(timeLayout, payload.OpenDate)
	if err != nil {
		return nil, apperrors.NewBadRequestError(fmt.Sprintf("Неверный формат даты: %s", payload.OpenDate))
	}

	entity := entities.Branch{
		Name:        payload.Name,
		ShortName:   payload.ShortName,
		Address:     payload.Address,
		PhoneNumber: payload.PhoneNumber,
		Email:       payload.Email,
		EmailIndex:  payload.EmailIndex,
		OpenDate:    openDate,
		StatusID:    payload.StatusID,
	}

	createdEntity, err := s.branchRepository.CreateBranch(ctx, entity)
	if err != nil {
		return nil, err
	}
	return branchEntityToDTO(createdEntity), nil
}

// Метод для обновления записи
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

	if payload.Name != nil {
		existingEntity.Name = *payload.Name
	}
	if payload.ShortName != nil {
		existingEntity.ShortName = *payload.ShortName
	}
	if payload.Address != nil {
		existingEntity.Address = *payload.Address
	}
	if payload.PhoneNumber != nil {
		existingEntity.PhoneNumber = *payload.PhoneNumber
	}
	if payload.Email != nil {
		existingEntity.Email = *payload.Email
	}
	if payload.EmailIndex != nil {
		existingEntity.EmailIndex = *payload.EmailIndex
	}
	if payload.StatusID != nil {
		existingEntity.StatusID = *payload.StatusID
	}
	if payload.OpenDate != nil {
		openDate, err := time.Parse(timeLayout, *payload.OpenDate)
		if err != nil {
			return nil, apperrors.NewBadRequestError(fmt.Sprintf("Неверный формат даты: %s", *payload.OpenDate))
		}
		existingEntity.OpenDate = openDate
	}

	updatedEntity, err := s.branchRepository.UpdateBranch(ctx, id, *existingEntity)
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
