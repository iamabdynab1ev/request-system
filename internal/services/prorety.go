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

func (s *PriorityService) GetPriorities(ctx context.Context, limit uint64, offset uint64) ([]dto.PriorityDTO, error) {
	return s.priorityRepository.GetPriorities(ctx, 1, 10)
}

func (s *PriorityService) FindPriority(ctx context.Context, id uint64) (*dto.PriorityDTO, error) {
	return s.priorityRepository.FindPriority(ctx, id)
}

func (s *PriorityService) CreatePriority(ctx context.Context, dto dto.CreatePriorityDTO) (*dto.PriorityDTO, error) {
	err := s.priorityRepository.CreatePriority(ctx, dto)
	if err != nil {
		s.logger.Error("Ощибка при создание свойства: ", zap.Error(err))
		return nil, err
	}
	s.logger.Info("Успешно создано", zap.Any("payload:", dto))
	return nil, err
}

func (s *PriorityService) UpdatePriority(ctx context.Context, id uint64, dto dto.UpdatePriorityDTO) (*dto.PriorityDTO, error) {
	err := s.priorityRepository.UpdatePriority(ctx, id, dto)
	if err != nil {
		return nil, err
	}

	return nil, err
}

func (s *PriorityService) DeletePriority(ctx context.Context, id uint64) error {
	return s.priorityRepository.DeletePriority(ctx, id)
}
