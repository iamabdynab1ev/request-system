package services

import (
	"context"
	"request-system/internal/dto"
	"request-system/internal/repositories"
	"request-system/pkg/contextkeys"
	apperrors "request-system/pkg/errors"

	"go.uber.org/zap"
)

type OrderServiceInterface interface {
	GetOrders(ctx context.Context, limit uint64, offset uint64) ([]dto.OrderDTO, uint64, error)
	FindOrder(ctx context.Context, id uint64) (*dto.OrderDTO, error)
	CreateOrder(ctx context.Context, orderData dto.CreateOrderDTO) (int, error)
	UpdateOrder(ctx context.Context, id uint64, orderData dto.UpdateOrderDTO) error
	DeleteOrder(ctx context.Context, id uint64) error
}

type OrderService struct {
	orderRepository repositories.OrderRepositoryInterface
	logger          *zap.Logger
}

func NewOrderService(
	orderRepository repositories.OrderRepositoryInterface,
	logger *zap.Logger,
) OrderServiceInterface {
	return &OrderService{
		orderRepository: orderRepository,
		logger:          logger,
	}
}

func (s *OrderService) GetOrders(ctx context.Context, limit uint64, offset uint64) ([]dto.OrderDTO, uint64, error) {
	orders, total, err := s.orderRepository.GetOrders(ctx, limit, offset)
	if err != nil {
		s.logger.Error("ошибка при получении списка заявок в сервисе", zap.Error(err))
		return nil, 0, err
	}
	return orders, total, nil
}

func (s *OrderService) FindOrder(ctx context.Context, id uint64) (*dto.OrderDTO, error) {
	order, err := s.orderRepository.FindOrder(ctx, id)
	if err != nil {
		s.logger.Error("ошибка при поиске заявки в сервисе", zap.Uint64("id", id), zap.Error(err))
		return nil, err
	}
	return order, nil
}

func (s *OrderService) CreateOrder(ctx context.Context, orderData dto.CreateOrderDTO) (int, error) {
	s.logger.Info("Запрос на создание заявки получен (сервис)", zap.Any("orderData", orderData))

	creatorUserIDInterface := ctx.Value(contextkeys.UserIDKey)
	if creatorUserIDInterface == nil {
		s.logger.Error("UserID не найден в контексте (сервис)")
		return 0, apperrors.ErrUserIDNotFoundInContext
	}

	creatorUserID, ok := creatorUserIDInterface.(int)
	if !ok || creatorUserID <= 0 {
		s.logger.Error("UserID в контексте имеет неверный тип или равен нулю")
		return 0, apperrors.ErrInvalidUserID
	}

	s.logger.Info("ID создателя заявки получен", zap.Int("creatorUserID", creatorUserID))

	newID, err := s.orderRepository.CreateOrder(ctx, creatorUserID, orderData)
	if err != nil {
		s.logger.Error("Ошибка репозитория при создании заявки", zap.Error(err))
		return 0, err
	}
	return newID, nil
}

func (s *OrderService) UpdateOrder(ctx context.Context, id uint64, dto dto.UpdateOrderDTO) error {
	// В будущем здесь может быть бизнес-логика:
	// - Проверка прав пользователя (может ли он обновлять эту заявку?)
	// - Отправка уведомлений и т.д.
	err := s.orderRepository.UpdateOrder(ctx, id, dto)
	if err != nil {
		s.logger.Error("Ошибка репозитория при обновлении заявки", zap.Uint64("id", id), zap.Error(err))
		return err
	}
	return nil
}

func (s *OrderService) DeleteOrder(ctx context.Context, id uint64) error {
	err := s.orderRepository.DeleteOrder(ctx, id)
	if err != nil {
		s.logger.Error("Ошибка репозитория при удалении заявки", zap.Uint64("id", id), zap.Error(err))
		return err
	}
	return nil
}
