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

func (s *BranchService) GetBranches(ctx context.Context, limit uint64, offset uint64) (interface{}, error) {
	res	, err := s.branchRepository.GetBranches(ctx, limit, offset)
	if err != nil {
		s.logger.Error("ошибка при получении списка филиалов", zap.Error(err))
		return nil, fmt.Errorf("failed to get branches: %w", err)
	}
	return res, nil
}

func (s *BranchService) FindBranch(ctx context.Context, id uint64) (*dto.BranchDTO, error) {
	res, err := s.branchRepository.FindBranch(ctx, id)

	if err != nil {
		s.logger.Error("ошибка при поиске филиала", zap.Uint64("id", id), zap.Error(err))
		return nil, fmt.Errorf("failed to find branch by id %d: %w", id, err)
	}
	return res, nil
}

func (s *BranchService) CreateBranch(ctx context.Context, payload dto.CreateBranchDTO) (uint64, error) {
	createdID, err := s.branchRepository.CreateBranch(ctx, payload)
	if err != nil {
		s.logger.Error("ошибка при создании филиала", zap.Error(err))
		return 0, fmt.Errorf("failed to create branch: %w", err)
	}

	s.logger.Info("Филиал успешно создан", zap.Uint64("id", createdID), zap.Any("payload:", payload))
	return createdID, nil
}

func (s *BranchService) UpdateBranch(ctx context.Context, id uint64, payload dto.UpdateBranchDTO) error {
	err := s.branchRepository.UpdateBranch(ctx, id, payload)
	if err != nil {
		s.logger.Error("ошибка при обновлении филиала", zap.Uint64("id", id), zap.Any("payload", payload), zap.Error(err))
		return fmt.Errorf("failed to update branch by id %d: %w", id, err)
	}

	s.logger.Info("Филиал успешно обновлен", zap.Uint64("id", id), zap.Any("payload", payload))
	return nil
}

func (s *BranchService) DeleteBranch(ctx context.Context, id uint64) error {
	err := s.branchRepository.DeleteBranch(ctx, id)
	if err != nil {
		s.logger.Error("ошибка при удалении филиала", zap.Uint64("id", id), zap.Error(err))
		return fmt.Errorf("failed to delete branch by id %d: %w", id, err)
	}

	s.logger.Info("Филиал успешно удален", zap.Uint64("id", id))
	return nil
}
