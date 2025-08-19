// Файл: internal/services/branch.go
// СКОПИРУЙТЕ И ПОЛНОСТЬЮ ЗАМЕНИТЕ СОДЕРЖИМОЕ
package services

import (
	"context"
	"fmt"
	"request-system/internal/dto"
	"request-system/internal/repositories"

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

// GetBranches выполняет преобразование данных для ответа
func (s *BranchService) GetBranches(ctx context.Context, limit, offset uint64) ([]dto.BranchListResponseDTO, uint64, error) {
	// 1. Получаем полные данные из репозитория
	branchesFromRepo, total, err := s.branchRepository.GetBranches(ctx, limit, offset)
	if err != nil {
		s.logger.Error("Ошибка в сервисе при получении филиалов", zap.Error(err))
		return nil, 0, fmt.Errorf("failed to get branches: %w", err)
	}

	// 2. Создаем слайс для ответа и преобразуем каждую запись
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
			OpenDate:    branch.OpenDate,
			CreatedAt:   branch.CreatedAt,
			UpdatedAt:   branch.UpdatedAt,
		}
		// Проверяем, что Status не nil, и извлекаем из него ID
		if branch.Status != nil {
			response.Status = branch.Status.ID
		}
		responseDTOs = append(responseDTOs, response)
	}

	// 3. Возвращаем преобразованный список
	return responseDTOs, total, nil
}

// FindBranch возвращает полную DTO (с объектом) для детального просмотра
func (s *BranchService) FindBranch(ctx context.Context, id uint64) (*dto.BranchDTO, error) {
	// ... (без изменений)
	res, err := s.branchRepository.FindBranch(ctx, id)
	if err != nil {
		s.logger.Error("ошибка при поиске филиала", zap.Uint64("id", id), zap.Error(err))
		return nil, err
	}
	return res, nil
}

// ... Остальные методы Create, Update, Delete без изменений
func (s *BranchService) CreateBranch(ctx context.Context, payload dto.CreateBranchDTO) (*dto.BranchDTO, error) {
	createdID, err := s.branchRepository.CreateBranch(ctx, payload)
	if err != nil {
		s.logger.Error("ошибка при создании филиала", zap.Error(err))
		return nil, fmt.Errorf("failed to create branch: %w", err)
	}
	s.logger.Info("Филиал успешно создан", zap.Uint64("id", createdID))
	return s.branchRepository.FindBranch(ctx, createdID)
}

func (s *BranchService) UpdateBranch(ctx context.Context, id uint64, payload dto.UpdateBranchDTO) (*dto.BranchDTO, error) {
	err := s.branchRepository.UpdateBranch(ctx, id, payload)
	if err != nil {
		s.logger.Error("ошибка при обновлении филиала", zap.Uint64("id", id), zap.Any("payload", payload), zap.Error(err))
		return nil, err
	}
	s.logger.Info("Филиал успешно обновлен", zap.Uint64("id", id))
	return s.branchRepository.FindBranch(ctx, id)
}

func (s *BranchService) DeleteBranch(ctx context.Context, id uint64) error {
	err := s.branchRepository.DeleteBranch(ctx, id)
	if err != nil {
		s.logger.Error("ошибка при удалении филиала", zap.Uint64("id", id), zap.Error(err))
		return err
	}
	s.logger.Info("Филиал успешно удален", zap.Uint64("id", id))
	return nil
}
