package services

import (
	"context"

	"request-system/internal/dto"
	"request-system/internal/repositories"
)

type BranchService struct {
	branchRepository repositories.BranchRepositoryInterface
}

func NewBranchService(branchRepository repositories.BranchRepositoryInterface) *BranchService {
	return &BranchService{
		branchRepository: branchRepository,
	}
}

func (s *BranchService) GetBranches(ctx context.Context, limit uint64, offset uint64) ([]dto.BranchDTO, error) {
	return s.branchRepository.GetBranches(ctx, 1, 10)
}

func (s *BranchService) FindBranch(ctx context.Context, id uint64) (*dto.BranchDTO, error) {
	return s.branchRepository.FindBranch(ctx, id)
}

func (s *BranchService) CreateBranch(ctx context.Context, dto dto.CreateBranchDTO) (*dto.BranchDTO, error) {
	err := s.branchRepository.CreateBranch(ctx, dto)
	if err != nil {
		return nil, err
	}

	return nil, err
}

func (s *BranchService) UpdateBranch(ctx context.Context, id uint64, dto dto.UpdateBranchDTO) (*dto.BranchDTO, error) {
	err := s.branchRepository.UpdateBranch(ctx, id, dto)
	if err != nil {
		return nil, err
	}

	return nil, err
}

func (s *BranchService) DeleteBranch(ctx context.Context, id uint64) error {
	return s.branchRepository.DeleteBranch(ctx, id)
}