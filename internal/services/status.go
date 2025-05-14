package services

import (
	"context"

	"request-system/internal/dto"
	"request-system/internal/repositories"
)

type StatusService struct {
	statusRepository repositories.StatusRepositoryInterface
}

func NewStatusService(statusRepository repositories.StatusRepositoryInterface) *StatusService {
	return &StatusService{
		statusRepository: statusRepository,
	}
}

func (s *StatusService) GetStatuses(ctx context.Context, limit uint64, offset uint64) ([]dto.StatusDTO, error) {
	return s.statusRepository.GetStatuses(ctx, 1, 10)
}

func (s *StatusService) FindStatus(ctx context.Context, id uint64) (*dto.StatusDTO, error) {
	return s.statusRepository.FindStatus(ctx, id)
}

func (s *StatusService) CreateStatus(ctx context.Context, dto dto.CreateStatusDTO) (*dto.StatusDTO, error) {
	err := s.statusRepository.CreateStatus(ctx, dto)
	if err != nil {
		return nil, err
	}

	return nil, err
}

func (s *StatusService) UpdateStatus(ctx context.Context, id uint64, dto dto.UpdateStatusDTO) (*dto.StatusDTO, error) {
	err := s.statusRepository.UpdateStatus(ctx, id, dto)
	if err != nil {
		return nil, err
	}

	return nil, err
}

func (s *StatusService) DeleteStatus(ctx context.Context, id uint64) error {
	return s.statusRepository.DeleteStatus(ctx, id)
}