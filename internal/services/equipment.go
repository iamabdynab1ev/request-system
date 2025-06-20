package services

import (
	"context"

	"request-system/internal/dto"
	"request-system/internal/repositories"

	"go.uber.org/zap"
)

type EquipmentService struct {
	equipmentRepository repositories.EquipmentRepositoryInterface
	logger              *zap.Logger
}

func NewEquipmentService(equipmentRepository repositories.EquipmentRepositoryInterface,
	logger *zap.Logger,
) *EquipmentService {
	return &EquipmentService{
		equipmentRepository: equipmentRepository,
		logger:              logger,
	}
}

func (s *EquipmentService) GetEquipments(ctx context.Context, limit uint64, offset uint64) (interface{}, uint64, error) {
	return s.equipmentRepository.GetEquipments(ctx, limit, offset)
}

func (s *EquipmentService) FindEquipment(ctx context.Context, id uint64) (*dto.EquipmentDTO, error) {
	return s.equipmentRepository.FindEquipment(ctx, id)
}

func (s *EquipmentService) CreateEquipment(ctx context.Context, dto dto.CreateEquipmentDTO) (*dto.EquipmentDTO, error) {
	err := s.equipmentRepository.CreateEquipment(ctx, dto)
	if err != nil {
		s.logger.Error("Ощибка при создание оборудования: ", zap.Error(err))
		return nil, err
	}
	s.logger.Info("Оборудование успешно создан", zap.Any("payload:", dto))
	return nil, err
}

func (s *EquipmentService) UpdateEquipment(ctx context.Context, id uint64, dto dto.UpdateEquipmentDTO) (*dto.EquipmentDTO, error) {
	err := s.equipmentRepository.UpdateEquipment(ctx, id, dto)
	if err != nil {
		return nil, err
	}

	return nil, err
}

func (s *EquipmentService) DeleteEquipment(ctx context.Context, id uint64) error {
	return s.equipmentRepository.DeleteEquipment(ctx, id)
}
