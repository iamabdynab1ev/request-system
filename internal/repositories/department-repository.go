package repositories

import (
	"context"

	"request-system/internal/dto"
	"request-system/internal/entities"
)

type DepartmentRepositoryInterface interface {
	GetDepartments(ctx context.Context, limit uint64, offset uint64) ([]entities.Department, error)
	FindDepartment(ctx context.Context, id uint64) (*entities.Department, error)
	CreateDepartment(ctx context.Context, payload dto.CreateDepartmentDTO) error
	UpdateDepartment(ctx context.Context, id uint64, payload dto.UpdateDepartmentDTO) error
	DeleteDepartment(ctx context.Context, id uint64) error
}

type DepartmentRepository struct{}

func NewDepartmentRepository() *DepartmentRepository {
	return &DepartmentRepository{}
}

func (r *DepartmentRepository) GetDepartments(ctx context.Context, limit uint64, offset uint64) ([]entities.Department, error) {
	return nil, nil
}

func (r *DepartmentRepository) FindDepartment(ctx context.Context, id uint64) (*entities.Department, error) {
	return nil, nil
}

func (r *DepartmentRepository) CreateDepartment(ctx context.Context, payload dto.CreateDepartmentDTO) error {
	return nil
}

func (r *DepartmentRepository) UpdateDepartment(ctx context.Context, id uint64, payload dto.UpdateDepartmentDTO) error {
	return nil
}

func (r *DepartmentRepository) DeleteDepartment(ctx context.Context, id uint64) error {
	return nil
}
