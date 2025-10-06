package services

/*
import (
	"context"
	"request-system/internal/dto"
	"request-system/internal/repositories"

	"go.uber.org/zap"
)

type OrderCommentServiceInterface interface {
	GetOrderComments(ctx context.Context, limit uint64, offset uint64) ([]dto.OrderCommentDTO, uint64, error)
	FindOrderComment(ctx context.Context, id uint64) (*dto.OrderCommentDTO, error)
	CreateOrderComment(ctx context.Context, dto dto.CreateOrderCommentDTO) (int, error)
	UpdateOrderComment(ctx context.Context, id uint64, dto dto.UpdateOrderCommentDTO) error
	DeleteOrderComment(ctx context.Context, id uint64) error
}

type OrderCommentService struct {
	orderCommentRepository repositories.OrderCommentRepositoryInterface
	logger                 *zap.Logger
}

func NewOrderCommentService(
	orderCommentRepository repositories.OrderCommentRepositoryInterface,
	logger *zap.Logger,
) OrderCommentServiceInterface {
	return &OrderCommentService{
		orderCommentRepository: orderCommentRepository,
		logger:                 logger,
	}
}

func (s *OrderCommentService) GetOrderComments(ctx context.Context, limit uint64, offset uint64) ([]dto.OrderCommentDTO, uint64, error) {
	// В будущем, если захочешь фильтровать по order_id, логика будет добавляться сюда.
	// Например, можно будет передавать карту фильтров.
	comments, total, err := s.orderCommentRepository.GetOrderComments(ctx, limit, offset)
	if err != nil {
		s.logger.Error("Ошибка получения списка комментариев в сервисе", zap.Error(err))
		return nil, 0, err
	}
	return comments, total, err
}

func (s *OrderCommentService) FindOrderComment(ctx context.Context, id uint64) (*dto.OrderCommentDTO, error) {
	comment, err := s.orderCommentRepository.FindOrderComment(ctx, id)
	if err != nil {
		s.logger.Error("Ошибка при поиске комментария в сервисе", zap.Uint64("id", id), zap.Error(err))
		return nil, err
	}
	return comment, nil
}

func (s *OrderCommentService) CreateOrderComment(ctx context.Context, dto dto.CreateOrderCommentDTO) (int, error) {
	newID, err := s.orderCommentRepository.CreateOrderComment(ctx, dto)
	if err != nil {
		s.logger.Error("Ошибка при создании комментария в сервисе", zap.Error(err), zap.Any("payload", dto))
		return 0, err
	}
	s.logger.Info("Комментарий успешно создан", zap.Int("newID", newID))
	return newID, nil
}

func (s *OrderCommentService) UpdateOrderComment(ctx context.Context, id uint64, dto dto.UpdateOrderCommentDTO) error {
	err := s.orderCommentRepository.UpdateOrderComment(ctx, id, dto)
	if err != nil {
		s.logger.Error("Ошибка при обновлении комментария в сервисе", zap.Uint64("id", id), zap.Error(err))
		return err
	}
	return nil
}

func (s *OrderCommentService) DeleteOrderComment(ctx context.Context, id uint64) error {
	err := s.orderCommentRepository.DeleteOrderComment(ctx, id)
	if err != nil {
		s.logger.Error("Ошибка при удалении комментария в сервисе", zap.Uint64("id", id), zap.Error(err))
		return err
	}
	return nil
}
*/
