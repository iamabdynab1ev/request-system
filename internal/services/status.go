package services

import (
	"context"

	"request-system/internal/dto"
	"request-system/internal/repositories"

	"go.uber.org/zap"
)

type StatusService struct {
	statusRepository repositories.StatusRepositoryInterface
	logger           *zap.Logger
}

func NewStatusService(statusRepository repositories.StatusRepositoryInterface,
	logger *zap.Logger,
) *StatusService {
	return &StatusService{
		statusRepository: statusRepository,
		logger:           logger,
	}
}

// GetStatuses passes the correct pagination parameters and returns a total count.
func (s *StatusService) GetStatuses(ctx context.Context, limit uint64, offset uint64) ([]dto.StatusDTO, uint64, error) {
	return s.statusRepository.GetStatuses(ctx, limit, offset)
}

func (s *StatusService) FindStatus(ctx context.Context, id uint64) (*dto.StatusDTO, error) {
	return s.statusRepository.FindStatus(ctx, id)
}

// FindByCode was missing and has been added.
func (s *StatusService) FindByCode(ctx context.Context, code string) (*dto.StatusDTO, error) {
	return s.statusRepository.FindByCode(ctx, code)
}

// CreateStatus now returns the created status DTO.
func (s *StatusService) CreateStatus(ctx context.Context, dto dto.CreateStatusDTO) (*dto.StatusDTO, error) {
	status, err := s.statusRepository.CreateStatus(ctx, dto)
	if err != nil {
		s.logger.Error("ошибка при создании статуса", zap.Error(err))
		return nil, err
	}
	s.logger.Info("Статус успешно создан", zap.Any("status", status))
	return status, nil
}

// UpdateStatus now returns the updated status DTO.
func (s *StatusService) UpdateStatus(ctx context.Context, id uint64, dto dto.UpdateStatusDTO) (*dto.StatusDTO, error) {
	status, err := s.statusRepository.UpdateStatus(ctx, id, dto)
	if err != nil {
		s.logger.Error("ошибка при обновлении статуса", zap.Error(err), zap.Uint64("id", id))
		return nil, err
	}
	s.logger.Info("Статус успешно обновлен", zap.Any("status", status))
	return status, nil
}

func (s *StatusService) DeleteStatus(ctx context.Context, id uint64) error {
	return s.statusRepository.DeleteStatus(ctx, id)
}
