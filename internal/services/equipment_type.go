package services

import (
	"context"

	"request-system/internal/dto"
	"request-system/internal/repositories"

	"go.uber.org/zap"
)

type EquipmentTypeService struct {
	equipmentTypeRepository repositories.EquipmentTypeRepositoryInterface
	logger                  *zap.Logger
}

func NewEquipmentTypeService(equipmentTypeRepository repositories.EquipmentTypeRepositoryInterface,
	logger *zap.Logger,
) *EquipmentTypeService {
	return &EquipmentTypeService{
		equipmentTypeRepository: equipmentTypeRepository,
		logger:                  logger,
	}
}

func (s *EquipmentTypeService) GetEquipmentTypes(ctx context.Context) ([]dto.EquipmentTypeDTO, error) {
	return s.equipmentTypeRepository.GetEquipmentTypes(ctx, 1, 10)
}

func (s *EquipmentTypeService) FindEquipmentType(ctx context.Context, id uint64) (*dto.EquipmentTypeDTO, error) {
	return s.equipmentTypeRepository.FindEquipmentType(ctx, id)
}

func (s *EquipmentTypeService) CreateEquipmentType(ctx context.Context, dto dto.CreateEquipmentTypeDTO) (*dto.EquipmentTypeDTO, error) {
	err := s.equipmentTypeRepository.CreateEquipmentType(ctx, dto)
	if err != nil {
		s.logger.Error("Ощибка при создание оборудования: ", zap.Error(err))

		return nil, err
	}
	s.logger.Info("Оборудование успешно создан", zap.Any("payload:", dto))
	return nil, err
}

func (s *EquipmentTypeService) UpdateEquipmentType(ctx context.Context, id uint64, dto dto.UpdateEquipmentTypeDTO) (*dto.EquipmentTypeDTO, error) {
	err := s.equipmentTypeRepository.UpdateEquipmentType(ctx, id, dto)
	if err != nil {
		return nil, err
	}

	return nil, err
}

func (s *EquipmentTypeService) DeleteEquipmentType(ctx context.Context, id uint64) error {
	return s.equipmentTypeRepository.DeleteEquipmentType(ctx, id)
}
