package repositories

import (
	"context"

	"request-system/internal/dto"
	"request-system/internal/entities"
)

type EquipmentTypeRepositoryInterface interface {
	GetEquipmentTypes(ctx context.Context, limit uint64, offset uint64) ([]entities.EquipmentType, error)
	FindEquipmentType(ctx context.Context, id uint64) (*entities.EquipmentType, error)
	CreateEquipmentType(ctx context.Context, payload dto.CreateEquipmentTypeDTO) error
	UpdateEquipmentType(ctx context.Context, id uint64, payload dto.UpdateEquipmentTypeDTO) error
	DeleteEquipmentType(ctx context.Context, id uint64) error
}

type EquipmentTypeRepository struct{}

func NewEquipmentTypeRepository() *EquipmentTypeRepository {
	return &EquipmentTypeRepository{}
}

func (r *EquipmentTypeRepository) GetEquipmentTypes(ctx context.Context, limit uint64, offset uint64) ([]entities.EquipmentType, error) {
	return nil, nil
}

func (r *EquipmentTypeRepository) FindEquipmentType(ctx context.Context, id uint64) (*entities.EquipmentType, error) {
	return nil, nil
}

func (r *EquipmentTypeRepository) CreateEquipmentType(ctx context.Context, payload dto.CreateEquipmentTypeDTO) error {
	return nil
}

func (r *EquipmentTypeRepository) UpdateEquipmentType(ctx context.Context, id uint64, payload dto.UpdateEquipmentTypeDTO) error {
	return nil
}

func (r *EquipmentTypeRepository) DeleteEquipmentType(ctx context.Context, id uint64) error {
	return nil
}
