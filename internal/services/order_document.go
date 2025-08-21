package services

import (
	"context"

	"request-system/internal/dto"
	"request-system/internal/repositories"
)

type OrderDocumentService struct {
	orderDocumentRepository repositories.OrderDocumentRepositoryInterface
}

func NewOrderDocumentService(orderDocumentRepository repositories.OrderDocumentRepositoryInterface) *OrderDocumentService {
	return &OrderDocumentService{
		orderDocumentRepository: orderDocumentRepository,
	}
}

func (s *OrderDocumentService) GetOrderDocuments(ctx context.Context, limit uint64, offset uint64) ([]dto.OrderDocumentDTO, error) {
	return s.orderDocumentRepository.GetOrderDocuments(ctx, 1, 10)
}

func (s *OrderDocumentService) FindOrderDocument(ctx context.Context, id uint64) (*dto.OrderDocumentDTO, error) {
	return s.orderDocumentRepository.FindOrderDocument(ctx, id)
}

func (s *OrderDocumentService) CreateOrderDocument(ctx context.Context, dto dto.CreateOrderDocumentDTO) (*dto.OrderDocumentDTO, error) {
	err := s.orderDocumentRepository.CreateOrderDocument(ctx, dto)
	if err != nil {
		return nil, err
	}

	return nil, err
}

func (s *OrderDocumentService) UpdateOrderDocument(ctx context.Context, id uint64, dto dto.UpdateOrderDocumentDTO) (*dto.OrderDocumentDTO, error) {
	err := s.orderDocumentRepository.UpdateOrderDocument(ctx, id, dto)
	if err != nil {
		return nil, err
	}

	return nil, err
}

func (s *OrderDocumentService) DeleteOrderDocument(ctx context.Context, id uint64) error {
	return s.orderDocumentRepository.DeleteOrderDocument(ctx, id)
}
