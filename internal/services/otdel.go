package services

import (
	"context"

	"request-system/internal/dto"
	"request-system/internal/repositories"
	"go.uber.org/zap"
)

type OtdelService struct {
	otdelRepository repositories.OtdelRepositoryInterface
	logger          *zap.Logger
}

func NewOtdelService(otdelRepository repositories.OtdelRepositoryInterface,
	logger  	*zap.Logger,
	) *OtdelService {
	return &OtdelService{
		otdelRepository: otdelRepository,
		logger : logger,
	}
}

func (s *OtdelService) GetOtdels(ctx context.Context, limit uint64, offset uint64) ([]dto.OtdelDTO, error) {
	return s.otdelRepository.GetOtdels(ctx, 1, 10)
}

func (s *OtdelService) FindOtdel(ctx context.Context, id uint64) (*dto.OtdelDTO, error) {
	return s.otdelRepository.FindOtdel(ctx, id)
}

func (s *OtdelService) CreateOtdel(ctx context.Context, dto dto.CreateOtdelDTO) (*dto.OtdelDTO, error) {
	err := s.otdelRepository.CreateOtdel(ctx, dto)
	if err != nil {
		s.logger.Error("Ощибка при создание отдел: ", zap.Error(err))
		return nil, err
	}
s.logger.Info("Отдел успешно создан", zap.Any("payload:", dto))
	return nil, err
}

func (s *OtdelService) UpdateOtdel(ctx context.Context, id uint64, dto dto.UpdateOtdelDTO) (*dto.OtdelDTO, error) {
	err := s.otdelRepository.UpdateOtdel(ctx, id, dto)
	if err != nil {
		return nil, err
	}

	return nil, err
}

func (s *OtdelService) DeleteOtdel(ctx context.Context, id uint64) error {
	return s.otdelRepository.DeleteOtdel(ctx, id)
}