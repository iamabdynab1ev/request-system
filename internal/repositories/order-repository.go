package repositories

import (
	"context"
	"request-system/internal/dto"
	"request-system/internal/entities"
)

type OrderRepositoryInterface interface {
	GetOrders(ctx context.Context, limit uint64, offset uint64) ([]entities.Order, error)
	FindOrder(ctx context.Context, id uint64) (*entities.Order, error)
	CreateOrder(ctx context.Context, payload dto.CreateOrderDTO) error
	UpdateOrder(ctx context.Context, id uint64, payload dto.UpdateOrderDTO) error
	DeleteOrder(ctx context.Context, id uint64) error
}

type OrderRepository struct{}

func NewOrderRepository() *OrderRepository {
	return &OrderRepository{}
}

func (r *OrderRepository) GetOrders(ctx context.Context, limit uint64, offset uint64) ([]entities.Order, error) {
	return []entities.Order{}, nil
}

func (r *OrderRepository) FindOrder(ctx context.Context, id uint64) (*entities.Order, error) {
	return nil, nil
}

func (r *OrderRepository) CreateOrder(ctx context.Context, payload dto.CreateOrderDTO) error {
	return nil
}

func (r *OrderRepository) UpdateOrder(ctx context.Context, id uint64, payload dto.UpdateOrderDTO) error {
	return nil
}

func (r *OrderRepository) DeleteOrder(ctx context.Context, id uint64) error {
	return nil
}
