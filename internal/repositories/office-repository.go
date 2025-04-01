package repositories

import (
	"context"

	"request-system/internal/dto"
	"request-system/internal/entities"
)
type OfficeRepositoriesInterface interface {
	CreateOffice(ctx context.Context, payload dto.CreateOfficeDTO) error 
	GetOffices(ctx context.Context, limit uint64, offset uint64) ([]entities.Office, error)
	FindOffice(ctx context.Context, id uint64) (*entities.Office, error)
	UpdateOffice(ctx context.Context, id uint64, payload dto.UpdateOfficeDTO) error 
	DeleteOffice(ctx context.Context, id uint64) error
	SyncOffice(ctx context.Context, id uint64, payload dto.OfficeDTO) error
}

type OfficeRepository struct {}

func NewOfficeRepository() *OfficeRepository {
	return &OfficeRepository{}
}

func (r *OfficeRepository) CreateOffice(ctx context.Context, payload dto.CreateOfficeDTO) error {
	return nil
}

func (r *OfficeRepository) GetOffices(ctx context.Context, limit uint64, offset uint64) ([]entities.Office, error) {
	return nil, nil
}

func (r *OfficeRepository) FindOffice(ctx context.Context, id uint64) (*entities.Office, error) {
	return nil, nil
}

func (r *OfficeRepository) UpdateOffice(ctx context.Context, id uint64, payload dto.UpdateOfficeDTO) error {
	return nil
}

func (r *OfficeRepository) DeleteOffice(ctx context.Context, id uint64) error {
	return nil
}

func (r *OfficeRepository) SyncOffice(ctx context.Context, id uint64, payload dto.OfficeDTO) error {	
	return nil
}
