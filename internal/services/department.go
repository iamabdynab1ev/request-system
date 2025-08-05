package services

import (
	"context"

	"request-system/internal/dto"
	"request-system/internal/repositories"
	"request-system/pkg/types"

	"go.uber.org/zap"
)

type DepartmentService struct {
	departmentRepository repositories.DepartmentRepositoryInterface
	logger               *zap.Logger
}

func NewDepartmentService(departmentRepository repositories.DepartmentRepositoryInterface, logger *zap.Logger) *DepartmentService {
	return &DepartmentService{
		departmentRepository: departmentRepository,
		logger:               logger,
	}
}

// GetDepartments теперь возвращает список, общее количество для пагинации и ошибку.
func (s *DepartmentService) GetDepartments(ctx context.Context, filter types.Filter) ([]dto.DepartmentDTO, uint64, error) {
	departments, err := s.departmentRepository.GetDepartments(ctx, filter)
	if err != nil {
		s.logger.Error("Ошибка при получении списка департаментов", zap.Error(err))
		return nil, 0, err
	}

	total, err := s.departmentRepository.CountDepartments(ctx, filter)
	if err != nil {
		s.logger.Error("Ошибка при подсчете количества департаментов", zap.Error(err))
		return nil, 0, err
	}

	return departments, total, nil
}

func (s *DepartmentService) FindDepartment(ctx context.Context, id uint64) (*dto.DepartmentDTO, error) {
	return s.departmentRepository.FindDepartment(ctx, id)
}

func (s *DepartmentService) CreateDepartment(ctx context.Context, dto dto.CreateDepartmentDTO) (*dto.DepartmentDTO, error) {
	department, err := s.departmentRepository.CreateDepartment(ctx, dto)
	if err != nil {
		s.logger.Error("Ошибка при создании департамента", zap.Error(err))
		return nil, err
	}

	s.logger.Info("Департамент успешно создан", zap.Any("department", department))
	return department, nil
}

func (s *DepartmentService) UpdateDepartment(ctx context.Context, id uint64, dto dto.UpdateDepartmentDTO) (*dto.DepartmentDTO, error) {
	department, err := s.departmentRepository.UpdateDepartment(ctx, id, dto)
	if err != nil {
		s.logger.Error("Ошибка при обновлении департамента", zap.Uint64("id", id), zap.Error(err))
		return nil, err
	}

	s.logger.Info("Департамент успешно обновлен", zap.Any("department", department))
	return department, nil
}

func (s *DepartmentService) DeleteDepartment(ctx context.Context, id uint64) error {
	err := s.departmentRepository.DeleteDepartment(ctx, id)
	if err != nil {
		s.logger.Error("Ошибка при удалении департамента", zap.Uint64("id", id), zap.Error(err))
	}
	return err
}
