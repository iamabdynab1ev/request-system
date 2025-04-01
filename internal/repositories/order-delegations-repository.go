package repositories

import (
	"context"

	"request-system/internal/dto"
	"request-system/internal/entities"
)

type OrderDelegationsRepositoryInterface interface {
	GetOrderDelegations(ctx context.Context, limit uint64, offset uint64) ([]entities.OrderDelegation, error)
	FindOrderDelegation(ctx context.Context, id uint64) (*entities.OrderDelegation, error)
	CreateOrderDelegation(ctx context.Context, payload dto.CreateOrderDelegationDTO) error
	UpdateOrderDelegation(ctx context.Context, id uint64, payload dto.CreateOrderDelegationDTO) error
	DeleteOrderDelegation(ctx context.Context, id uint64) error
}

type OrderDelegationRepository struct{}

func NewOrderDelegationsRepository() *OrderDelegationRepository {
	return &OrderDelegationRepository{}
}

func (r *OrderDelegationRepository) GetOrderDelegations(ctx context.Context, limit uint64, offset uint64) ([]entities.OrderDelegation, error) {
	return []entities.OrderDelegation{}, nil
}

func (r *OrderDelegationRepository) FindOrderDelegation(ctx context.Context, id uint64) (*entities.OrderDelegation, error) {
	return nil, nil
}

func (r *OrderDelegationRepository) CreateOrderDelegation(ctx context.Context, payload dto.CreateOrderDelegationDTO) error {
	return nil
}

func (r *OrderDelegationRepository) UpdateOrderDelegation(ctx context.Context, id uint64, payload dto.CreateOrderDelegationDTO) error {
	return nil
}

func (r *OrderDelegationRepository) DeleteOrderDelegation(ctx context.Context, id uint64) error {
	return nil
}
