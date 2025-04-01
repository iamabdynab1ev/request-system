package repositories

import (
	"context"

	"request-system/internal/dto"
	"request-system/internal/entities"
)
type BranchRepositoryInterface interface {
	GetBranches(ctx context.Context, limit uint64, offset uint64 ) ([] entities.Branch, error) 
	FindBranch(ctx context.Context, id uint64) (*entities.Branch, error)
	CreateBranch(ctx context.Context, payload dto.CreateBranchDTO) error 
	UpdateBranch(ctx context.Context, id uint64, payload dto.UpdateBranchDTO) error 
	DeleteBranch(ctx context.Context, id uint64) error
}

type BranchRepository struct {}

func NewBranchRepository() *BranchRepository {
	return &BranchRepository{}
}

func (r *BranchRepository) GetBranches(ctx context.Context, limit uint64, offset uint64) ([]entities.Branch, error) {
	return []entities.Branch{}, nil
}

func (r *BranchRepository) FindBranch(ctx context.Context, id uint64) (*entities.Branch, error) {
	return nil, nil
}

func (r *BranchRepository) CreateBranch(ctx context.Context, payload dto.CreateBranchDTO) error{
	return  nil
}

func (r *BranchRepository) UpdateBranch(ctx context.Context, id uint64, payload dto.UpdateBranchDTO) error {
	return nil
}

func (r *BranchRepository) DeleteBranch(ctx context.Context, id uint64) error {
	return nil
}


