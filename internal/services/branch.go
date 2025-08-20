package services

import (
	"context"
	"fmt"
	"request-system/internal/dto"
	"request-system/internal/entities" // <-- Важно: импортируем entities
	"request-system/internal/repositories"
	"request-system/pkg/types" // <-- Важно: импортируем types для filter
	"time"

	"go.uber.org/zap"
)

type BranchService struct {
	branchRepository repositories.BranchRepositoryInterface
	logger           *zap.Logger
}

func NewBranchService(
	branchRepository repositories.BranchRepositoryInterface,
	logger *zap.Logger,
) *BranchService {
	return &BranchService{
		branchRepository: branchRepository,
		logger:           logger,
	}
}

// НОВЫЙ ХЕЛПЕР для конвертации одной Entity в детальную DTO
func branchEntityToDTO(entity *entities.Branch) *dto.BranchDTO {
	if entity == nil {
		return nil
	}

	dtoStatus := &dto.ShortStatusDTO{}
	if entity.Status != nil {
		dtoStatus.ID = uint64(entity.Status.ID) // >>> ИЗМЕНЕНИЕ: Просто добавляем uint64() <<<
		dtoStatus.Name = entity.Status.Name
	}

	return &dto.BranchDTO{
		ID:          entity.ID,
		Name:        entity.Name,
		ShortName:   entity.ShortName,
		Address:     entity.Address,
		PhoneNumber: entity.PhoneNumber,
		Email:       entity.Email,
		EmailIndex:  entity.EmailIndex,
		OpenDate:    entity.OpenDate.Format("2006-01-02"),
		Status:      dtoStatus,
		CreatedAt:   entity.CreatedAt.Format("2006-01-02 15:04:05"),
		UpdatedAt:   entity.UpdatedAt.Format("2006-01-02 15:04:05"),
	}
}

// GetBranches теперь принимает filter и возвращает DTO для списка
func (s *BranchService) GetBranches(ctx context.Context, filter types.Filter) ([]dto.BranchListResponseDTO, uint64, error) {
	branchesFromRepo, total, err := s.branchRepository.GetBranches(ctx, filter)
	if err != nil {
		s.logger.Error("Ошибка в сервисе при получении филиалов", zap.Error(err))
		return nil, 0, err
	}

	responseDTOs := make([]dto.BranchListResponseDTO, 0, len(branchesFromRepo))
	for _, branch := range branchesFromRepo {
		response := dto.BranchListResponseDTO{
			ID:          branch.ID,
			Name:        branch.Name,
			ShortName:   branch.ShortName,
			Address:     branch.Address,
			PhoneNumber: branch.PhoneNumber,
			Email:       branch.Email,
			EmailIndex:  branch.EmailIndex,
			OpenDate:    branch.OpenDate.Format("2006-01-02"),
			CreatedAt:   branch.CreatedAt.Format("2006-01-02 15:04:05"),
			UpdatedAt:   branch.UpdatedAt.Format("2006-01-02 15:04:05"),
		}
		if branch.Status != nil {
			response.StatusID = uint64(branch.Status.ID) // >>> ИЗДМЕНЕНИЕ: И здесь тоже <<<
		}
		responseDTOs = append(responseDTOs, response)
	}

	return responseDTOs, total, nil
}

// FindBranch теперь конвертирует entity в DTO
func (s *BranchService) FindBranch(ctx context.Context, id uint64) (*dto.BranchDTO, error) {
	res, err := s.branchRepository.FindBranch(ctx, id)
	if err != nil {
		s.logger.Error("ошибка при поиске филиала", zap.Uint64("id", id), zap.Error(err))
		return nil, err
	}
	// >>> ИЗМЕНЕНИЕ: Конвертируем entity в DTO перед возвратом <<<
	return branchEntityToDTO(res), nil
}

// CreateBranch конвертирует DTO в entity
func (s *BranchService) CreateBranch(ctx context.Context, payload dto.CreateBranchDTO) (*dto.BranchDTO, error) {
	openDate, err := time.Parse("2006-01-02", payload.OpenDate)
	if err != nil {
		return nil, fmt.Errorf("неверный формат даты: %w", err)
	}

	branchEntity := entities.Branch{
		Name:        payload.Name,
		ShortName:   payload.ShortName,
		Address:     payload.Address,
		PhoneNumber: payload.PhoneNumber,
		Email:       payload.Email,
		EmailIndex:  payload.EmailIndex,
		OpenDate:    openDate,
		StatusID:    payload.StatusID,
	}

	createdID, err := s.branchRepository.CreateBranch(ctx, branchEntity)
	if err != nil {
		return nil, err
	}

	// Возвращаем свежесозданный объект, уже сконвертированный в DTO
	return s.FindBranch(ctx, createdID)
}

// UpdateBranch ищет entity, обновляет её из DTO и сохраняет
func (s *BranchService) UpdateBranch(ctx context.Context, id uint64, payload dto.UpdateBranchDTO) (*dto.BranchDTO, error) {
	existingBranch, err := s.branchRepository.FindBranch(ctx, id)
	if err != nil {
		return nil, err
	}

	// Обновляем только те поля, что пришли в DTO
	if payload.Name != "" {
		existingBranch.Name = payload.Name
	}
	if payload.ShortName != "" {
		existingBranch.ShortName = payload.ShortName
	}
	// ... и так далее для всех полей ...
	if payload.StatusID != 0 {
		existingBranch.StatusID = payload.StatusID
	}
	if payload.OpenDate != "" {
		openDate, err := time.Parse("2006-01-02", payload.OpenDate)
		if err != nil {
			return nil, err
		}
		existingBranch.OpenDate = openDate
	}

	err = s.branchRepository.UpdateBranch(ctx, id, *existingBranch)
	if err != nil {
		return nil, err
	}
	return s.FindBranch(ctx, id)
}

// DeleteBranch остается без изменений
func (s *BranchService) DeleteBranch(ctx context.Context, id uint64) error {
	err := s.branchRepository.DeleteBranch(ctx, id)
	if err != nil {
		s.logger.Error("ошибка при удалении филиала", zap.Uint64("id", id), zap.Error(err))
		return err
	}
	return nil
}
