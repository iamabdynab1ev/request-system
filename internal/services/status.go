package services

import (
	"context"

	"request-system/internal/dto"
	"request-system/internal/repositories"
)

type StatusService struct {
	statusRepository repositories.StatusRepositoryInterface
}

func NewStatusService(
	statusRepository repositories.StatusRepositoryInterface,
) *StatusService {
	return &StatusService{
		statusRepository: statusRepository,
	}
}

func (service *StatusService) GetAll(ctx context.Context) ([]dto.StatusDTO, error) {
	return service.statusRepository.GetStatuses(ctx, 1, 10)
}
