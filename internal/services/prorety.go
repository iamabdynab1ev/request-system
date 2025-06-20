package services

import (
	"context"

	"request-system/internal/dto"
	"go.uber.org/zap"
	"request-system/internal/repositories"
)

type ProretyService struct {
	proretyRepository repositories.ProretyRepositoryInterface
	logger            *zap.Logger
}

func NewProretyService(proretyRepository repositories.ProretyRepositoryInterface,
	logger 	*zap.Logger,
	) *ProretyService {
	return &ProretyService{
		proretyRepository: proretyRepository,
		logger: logger,
		}
}

func (s *ProretyService) GetProreties(ctx context.Context, limit uint64, offset uint64) ([]dto.ProretyDTO, error) {
	return s.proretyRepository.GetProreties(ctx, 1, 10)
}

func (s *ProretyService) FindProrety(ctx context.Context, id uint64) (*dto.ProretyDTO, error) {
	return s.proretyRepository.FindProrety(ctx, id)
}

func (s *ProretyService) CreateProrety(ctx context.Context, dto dto.CreateProretyDTO) (*dto.ProretyDTO, error) {
	err := s.proretyRepository.CreateProrety(ctx, dto)
	if err != nil {
		s.logger.Error("Ощибка при создание свойства: ", zap.Error(err))
		return nil, err
	}
 	s.logger.Info("Успешно создано", zap.Any("payload:", dto))
	return nil, err
}

func (s *ProretyService) UpdateProrety(ctx context.Context, id uint64, dto dto.UpdateProretyDTO) (*dto.ProretyDTO, error) {
	err := s.proretyRepository.UpdateProrety(ctx, id, dto)
	if err != nil {
		return nil, err
	}

	return nil, err
}

func (s *ProretyService) DeleteProrety(ctx context.Context, id uint64) error {
	return s.proretyRepository.DeleteProrety(ctx, id)
}