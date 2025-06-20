package services

import (
	"context"

	"request-system/internal/dto"
	"request-system/internal/repositories"
	"go.uber.org/zap"
)

type DepartmentService struct {
	departmentRepository repositories.DepartmentRepositoryInterface
	logger 					*zap.Logger
}

func NewDepartmentService(departmentRepository repositories.DepartmentRepositoryInterface,
	logger *zap.Logger,
)*DepartmentService {
	return &DepartmentService{
		departmentRepository: departmentRepository,
		logger:               logger,
	}

}

func (s *DepartmentService) GetDepartments(ctx context.Context, limit uint64, offset uint64) ([]dto.DepartmentDTO, error) {
	return s.departmentRepository.GetDepartments(ctx, 1, 10)
}

func (s *DepartmentService) FindDepartment(ctx context.Context, id uint64) (*dto.DepartmentDTO, error) {
	return s.departmentRepository.FindDepartment(ctx, id)
}

func (s *DepartmentService) CreateDepartment(ctx context.Context, dto dto.CreateDepartmentDTO) (*dto.DepartmentDTO, error) {
	err := s.departmentRepository.CreateDepartment(ctx, dto)
	if err != nil {
		s.logger.Error("Ощибка при создание департамента: ", zap.Error(err))
		return nil, err
	}
	
	s.logger.Info("Департамент успешно создан", zap.Any("payload:", dto))
	return nil, err
}

func (s *DepartmentService) UpdateDepartment(ctx context.Context, id uint64, dto dto.UpdateDepartmentDTO) (*dto.DepartmentDTO, error) {
	err := s.departmentRepository.UpdateDepartment(ctx, id, dto)
	if err != nil {
		return nil, err
	}

	return nil, err
}

func (s *DepartmentService) DeleteDepartment(ctx context.Context, id uint64) error {
	return s.departmentRepository.DeleteDepartment(ctx, id)
}