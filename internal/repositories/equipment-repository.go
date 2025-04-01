package repositories

import (
	"context"
	"request-system/internal/dto"
	"request-system/internal/entities"
)

type EquipmentRepositoryInterface interface {
	GetEquipments(ctx context.Context , limit uint64, offset uint64) ([]entities.Equipment, error)
	FindEquipment(ctx context.Context, id uint64) (*entities.Equipment, error)
	CreateEquipment(ctx context.Context, payload dto.CreateEquipmentDTO) error
	UpdateEquipment(ctx context.Context, id uint64, payload dto.UpdateEquipmentDTO) error
	DeleteEquipment(ctx context.Context, id uint64) error
	}


	type EquipmentRepository struct {}


	func NewEquipmentRepository() *EquipmentRepository {
		return &EquipmentRepository {} 
	}

	func (r *EquipmentRepository) GetEquipments(ctx context.Context, limit uint64, offset uint64) ([]entities.Equipment, error) {
		return nil, nil 
	}
	func (r *EquipmentRepository) FindEquipment(ctx context.Context, id uint64) (*entities.Equipment, error) {
		return nil, nil
	} 

	func (r *EquipmentRepository) CreateEquipment(ctx context.Context, payload dto.CreateEquipmentDTO) error {
		return nil
	}

	func (r *EquipmentRepository) UpdateEquipment(ctx context.Context, id uint64, payload dto.UpdateEquipmentDTO) error {
		return nil
	}

	func (r *EquipmentRepository) DeleteEquipment(ctx context.Context, id uint64) error {
		return nil
	}
	