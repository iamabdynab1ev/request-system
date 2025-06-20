package services

import (
	"context"

	"request-system/internal/dto"
	"request-system/internal/repositories"

	"go.uber.org/zap"

)

type StatusService struct {
	statusRepository repositories.StatusRepositoryInterface
	logger      *zap.Logger
}

func NewStatusService(statusRepository repositories.StatusRepositoryInterface,
	logger *zap.Logger,
	) *StatusService {
	return &StatusService{
		statusRepository: statusRepository,
		logger : logger,
	}
}

func (s *StatusService) GetStatuses(ctx context.Context, limit uint64, offset uint64) ([]dto.StatusDTO, error) {
	return s.statusRepository.GetStatuses(ctx, 1, 10)
}

func (s *StatusService) FindStatus(ctx context.Context, id uint64) (*dto.StatusDTO, error) {
	return s.statusRepository.FindStatus(ctx, id)
}

func (s *StatusService) CreateStatus(ctx context.Context, dto dto.CreateStatusDTO) (*dto.StatusDTO, error) {
	err := s.statusRepository.CreateStatus(ctx, dto)
	if err != nil {
		s.logger.Error("ошибка при создании статуса", zap.Error(err))
		return nil, err
	}
s.logger.Info("Статус успешно создан", zap.Any("payload:", dto))
	return nil, err
}

func (s *StatusService) UpdateStatus(ctx context.Context, id uint64, dto dto.UpdateStatusDTO) (*dto.StatusDTO, error) {
	err := s.statusRepository.UpdateStatus(ctx, id, dto)
	if err != nil {
		return nil, err
	}

	return nil, err
}

func (s *StatusService) DeleteStatus(ctx context.Context, id uint64) error {
	return s.statusRepository.DeleteStatus(ctx, id)
}
