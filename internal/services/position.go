package services

import (
	"context"

	"request-system/internal/dto"
	"request-system/internal/repositories"

	"go.uber.org/zap"
)

type PositionService struct {
	positionRepository repositories.PositionRepositoryInterface
	logger             *zap.Logger
}

func NewPositionService(positionRepository repositories.PositionRepositoryInterface,
	logger *zap.Logger,
) *PositionService {
	return &PositionService{
		positionRepository: positionRepository,
		logger:             logger,
	}
}

func (s *PositionService) GetPositions(ctx context.Context, limit uint64, offset uint64) (interface{}, uint64, error) {
	return s.positionRepository.GetPositions(ctx, limit, offset)
}

func (s *PositionService) FindPosition(ctx context.Context, id uint64) (*dto.PositionDTO, error) {
	data, err := s.positionRepository.FindPosition(ctx, id)
	if err != nil {
		s.logger.Error("Ощибка при поиске оборудования: ", zap.Error(err))
		return nil, err
	}

	return data, err
}

func (s *PositionService) CreatePosition(ctx context.Context, dto dto.CreatePositionDTO) (*dto.PositionDTO, error) {
	err := s.positionRepository.CreatePosition(ctx, dto)
	if err != nil {
		s.logger.Error("Ощибка при создание оборудования: ", zap.Error(err))
		return nil, err
	}
	s.logger.Info("Оборудование успешно создан", zap.Any("payload:", dto))
	return nil, err
}

func (s *PositionService) UpdatePosition(ctx context.Context, id uint64, dto dto.UpdatePositionDTO) (*dto.PositionDTO, error) {
	err := s.positionRepository.UpdatePosition(ctx, id, dto)
	if err != nil {
		return nil, err
	}

	return nil, err
}

func (s *PositionService) DeletePosition(ctx context.Context, id uint64) error {
	return s.positionRepository.DeletePosition(ctx, id)
}
