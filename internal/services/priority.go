package services

import (
	"context"

	"request-system/internal/dto"
	"request-system/internal/repositories"

	"go.uber.org/zap"
)

type PriorityService struct {
	priorityRepository repositories.PriorityRepositoryInterface
	logger             *zap.Logger
}

func NewPriorityService(priorityRepository repositories.PriorityRepositoryInterface,
	logger *zap.Logger,
) *PriorityService {
	return &PriorityService{
		priorityRepository: priorityRepository,
		logger:             logger,
	}
}

// Updated to pass pagination parameters and return the total count.
func (s *PriorityService) GetPriorities(ctx context.Context, limit uint64, offset uint64) ([]dto.PriorityDTO, uint64, error) {
	priorities, total, err := s.priorityRepository.GetPriorities(ctx, limit, offset)
	if err != nil {
		s.logger.Error("Error fetching priorities", zap.Error(err))
		return nil, 0, err
	}
	return priorities, total, nil
}

func (s *PriorityService) FindPriority(ctx context.Context, id uint64) (*dto.PriorityDTO, error) {
	return s.priorityRepository.FindPriority(ctx, id)
}

// Updated to return the created DTO.
func (s *PriorityService) CreatePriority(ctx context.Context, dto dto.CreatePriorityDTO) (*dto.PriorityDTO, error) {
	createdPriority, err := s.priorityRepository.CreatePriority(ctx, dto)
	if err != nil {
		s.logger.Error("Error creating priority", zap.Error(err))
		return nil, err
	}
	s.logger.Info("Successfully created priority", zap.Uint64("id", createdPriority.ID))
	return createdPriority, nil
}

func (s *PriorityService) UpdatePriority(ctx context.Context, id uint64, dto dto.UpdatePriorityDTO) (*dto.PriorityDTO, error) {
	updatedPriority, err := s.priorityRepository.UpdatePriority(ctx, id, dto)
	if err != nil {
		s.logger.Error("Error updating priority", zap.Error(err), zap.Uint64("id", id))
		return nil, err
	}
	s.logger.Info("Successfully updated priority", zap.Uint64("id", updatedPriority.ID))
	return updatedPriority, nil
}

func (s *PriorityService) DeletePriority(ctx context.Context, id uint64) error {
	err := s.priorityRepository.DeletePriority(ctx, id)
	if err != nil {
		s.logger.Error("Error deleting priority", zap.Error(err), zap.Uint64("id", id))
	}
	return err
}
