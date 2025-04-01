package repositories

import (
	"context"

	"request-system/internal/dto"
	"request-system/internal/entities"
)

type ProretyRepositoryInterface interface {
	GetProreties(ctx context.Context, limit uint64, offset uint64) ([]entities.Prorety, error)
	FindProrety(ctx context.Context, id uint64) (*entities.Prorety, error)
	CreateProrety(ctx context.Context, payload dto.CreateProretyDTO) error
	UpdateProrety(ctx context.Context, id uint64, payload dto.UpdateProretyDTO) error
	DeleteProrety(ctx context.Context, id uint64) error
}

type ProretyRepository struct{}

func NewProretyRepository() *ProretyRepository {
	return &ProretyRepository{}
}

func (r *ProretyRepository) GetProreties(ctx context.Context, limit uint64, offset uint64) ([]entities.Prorety, error) {
	return []entities.Prorety{}, nil
}

func (r *ProretyRepository) FindProrety(ctx context.Context, id uint64) (*entities.Prorety, error) {
	return nil, nil
}

func (r *ProretyRepository) CreateProrety(ctx context.Context, payload dto.CreateProretyDTO) error {
	return nil
}

func (r *ProretyRepository) UpdateProrety(ctx context.Context, id uint64, payload dto.UpdateProretyDTO) error {
	return nil
}

func (r *ProretyRepository) DeleteProrety(ctx context.Context, id uint64) error {
	return nil
}
