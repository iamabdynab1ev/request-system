package repositories

import (
	"context"

	"request-system/internal/dto"
	"request-system/internal/entities"
)

type OrderCommentRepositoryInterface interface {
	GetOrderComments(ctx context.Context, limit uint64, offset uint64) ([]entities.OrderComments, error)
	FindOrderComment(ctx context.Context, id uint64) (*entities.OrderComments, error)
	CreateOrderComment(ctx context.Context, payload dto.CreateOrderCommentDTO) error
	UpdateOrderComment(ctx context.Context, id uint64,  payload dto.CreateOrderCommentDTO) error
	DeleteOrderComment(ctx context.Context, id uint64) error
}

type OrderCommentsRepository struct{}

func NewOrderCommentsRepository() *OrderCommentsRepository {
	return &OrderCommentsRepository{} 
}

func (r *OrderCommentsRepository) GetOrderComments(ctx context.Context, limit uint64, offset uint64) ([]entities.OrderComments, error) {
	return []entities.OrderComments{}, nil
}

func (r *OrderCommentsRepository) FindOrderComment(ctx context.Context, id uint64) (*entities.OrderComments, error) {
	return nil, nil
}

func (r *OrderCommentsRepository) CreateOrderComment(ctx context.Context,  payload dto.CreateOrderCommentDTO) error {
	return nil
}

func (r *OrderCommentsRepository) UpdateOrderComment(ctx context.Context, id uint64,  payload dto.CreateOrderCommentDTO) error {
	return nil
}

func (r *OrderCommentsRepository) DeleteOrderComment(ctx context.Context, id uint64) error {
	return nil
}

