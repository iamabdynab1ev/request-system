package repositories

import (
	"context"

	"request-system/internal/dto"
	"request-system/internal/entities"
)

type OtdelRepositoryInterface interface {
	GetOtdels(ctx context.Context, limit uint64, offset uint64) ([]entities.Otdel, error)
	FindOtdel(ctx context.Context, id uint64) (*entities.Otdel, error)
	CreateOtdel(ctx context.Context, payload dto.CreateOtdelDTO) error
	UpdateOtdel(ctx context.Context, id uint64, payload dto.UpdateOtdelDTO) error
	DeleteOtdel(ctx context.Context, id uint64) error
}

type OtdelRepository struct{}

func NewOtdelRepository() *OtdelRepository {
	return &OtdelRepository{}
}

func (r *OtdelRepository) GetOtdels(ctx context.Context, limit uint64, offset uint64) ([]entities.Otdel, error) {
	return []entities.Otdel{}, nil
}

func (r *OtdelRepository) FindOtdel(ctx context.Context, id uint64) (*entities.Otdel, error) {
	return nil, nil
}

func (r *OtdelRepository) CreateOtdel(ctx context.Context, payload dto.CreateOtdelDTO) error {
	return nil
}

func (r *OtdelRepository) UpdateOtdel(ctx context.Context, id uint64, payload dto.UpdateOtdelDTO) error {
	return nil
}

func (r *OtdelRepository) DeleteOtdel(ctx context.Context, id uint64) error {
	return nil
}
