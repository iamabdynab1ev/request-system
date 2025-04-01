package repositories

import (
	"context"

	"request-system/internal/dto"
	"request-system/internal/entities"
)

type OrderDocumentsRepositoryInterface interface {
	GetOrderDocument(ctx context.Context, limit uint64, offset uint64) ([]entities.OrderDocuments, error)
	FindOrderDocument(ctx context.Context, id uint64) (*entities.OrderDelegation, error)
	CreateOrderDocument(ctx context.Context, payload dto.CreateOrderDocumentDTO) error
	UpdateOrderDocument(ctx context.Context, id uint64, payload dto.CreateOrderDocumentDTO) error
	DeleteOrderDocument(ctx context.Context, id uint64) error
}

type OrderDocumentRepository struct{}

func NewOrderDocumentsRepository() *OrderDocumentRepository {
	return &OrderDocumentRepository{}
}

func (r *OrderDocumentRepository) GetOrderDocuments(ctx context.Context, limit uint64, offset uint64) ([]entities.OrderDocuments, error) {
	return []entities.OrderDocuments{}, nil
}

func (r *OrderDocumentRepository) FindOrderDocument(ctx context.Context, id uint64) (*entities.OrderDocuments, error) {
	return nil, nil
}

func (r *OrderDocumentRepository) CreateOrderDocument(ctx context.Context, payload dto.CreateOrderDocumentDTO) error {
	return nil
}

func (r *OrderDocumentRepository) UpdateOrderDocument(ctx context.Context, id uint64, payload dto.CreateOrderDocumentDTO) error {
	return nil
}

func (r *OrderDocumentRepository) DeleteOrderDocument(ctx context.Context, id uint64) error {
	return nil
}
