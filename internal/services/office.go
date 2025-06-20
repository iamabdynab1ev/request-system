package services

import (
	"context"

	"request-system/internal/dto"
	"request-system/internal/repositories"

	"go.uber.org/zap"
)

type OfficeService struct {
	officeRepository repositories.OfficeRepositoryInterface
	logger           *zap.Logger
}

func NewOfficeService(officeRepository repositories.OfficeRepositoryInterface,
	logger *zap.Logger,
) *OfficeService {
	return &OfficeService{
		officeRepository: officeRepository,
		logger:           logger,
	}
}

func (s *OfficeService) GetOffices(ctx context.Context, limit uint64, offset uint64) ([]dto.OfficeDTO, error) {
	offices, err := s.officeRepository.GetOffices(ctx, 1, 10)
	if err != nil {
		s.logger.Error("Ощибка при получение офисов: ", zap.Error(err))
		return nil, err
	}
	return offices, nil
}

func (s *OfficeService) FindOffice(ctx context.Context, id uint64) (*dto.OfficeDTO, error) {
	return s.officeRepository.FindOffice(ctx, id)
}

func (s *OfficeService) CreateOffice(ctx context.Context, dto dto.CreateOfficeDTO) (*dto.OfficeDTO, error) {
	err := s.officeRepository.CreateOffice(ctx, dto)
	if err != nil {
		s.logger.Error("Ощибка при создание офис: ", zap.Error(err))
		return nil, err
	}
	s.logger.Info("Офис успешно создан", zap.Any("payload:", dto))
	return nil, err
}

func (s *OfficeService) UpdateOffice(ctx context.Context, id uint64, dto dto.UpdateOfficeDTO) (*dto.OfficeDTO, error) {
	err := s.officeRepository.UpdateOffice(ctx, id, dto)
	if err != nil {
		s.logger.Error("Ощибка при обновление офиса: ", zap.Error(err))
		return nil, err
	}

	s.logger.Info("Офис успешно обновлен", zap.Any("payload:", dto))

	return nil, err
}

func (s *OfficeService) DeleteOffice(ctx context.Context, id uint64) error {
	s.logger.Info("Офис успешно удален", zap.Uint64("id", id))
	return s.officeRepository.DeleteOffice(ctx, id)
}
