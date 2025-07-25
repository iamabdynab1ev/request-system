package services
/*
import (
	"context"
	"request-system/internal/dto"
	"request-system/internal/repositories"

	"go.uber.org/zap"
)


type OrderDelegationServiceInterface interface {
	GetOrderDelegations(ctx context.Context, limit uint64, offset uint64) ([]dto.OrderDelegationDTO, uint64, error)
	FindOrderDelegation(ctx context.Context, id uint64) (*dto.OrderDelegationDTO, error)
	CreateOrderDelegation(ctx context.Context, payload dto.CreateOrderDelegationDTO) (int, error)
	DeleteOrderDelegation(ctx context.Context, id uint64) error
}

type OrderDelegationService struct {
	orderDelegationRepository repositories.OrderDelegationRepositoryInterface
	logger                    *zap.Logger
}

func NewOrderDelegationService(
	repo repositories.OrderDelegationRepositoryInterface,
	logger *zap.Logger,
) OrderDelegationServiceInterface {
	return &OrderDelegationService{
		orderDelegationRepository: repo,
		logger:                    logger,
	}
}

func (s *OrderDelegationService) GetOrderDelegations(ctx context.Context, limit uint64, offset uint64) ([]dto.OrderDelegationDTO, uint64, error) {
	// Если нужно будет фильтровать по order_id, это будет делаться здесь,
	// но пока просто получаем все с пагинацией.
	delegations, total, err := s.orderDelegationRepository.GetOrderDelegations(ctx, limit, offset)
	if err != nil {
		s.logger.Error("Ошибка получения списка делегирований в сервисе", zap.Error(err))
		return nil, 0, err
	}
	return delegations, total, err
}

func (s *OrderDelegationService) FindOrderDelegation(ctx context.Context, id uint64) (*dto.OrderDelegationDTO, error) {
	delegation, err := s.orderDelegationRepository.FindOrderDelegation(ctx, id)
	if err != nil {
		s.logger.Error("Ошибка поиска делегирования в сервисе", zap.Error(err), zap.Uint64("id", id))
		return nil, err
	}
	return delegation, nil
}

func (s *OrderDelegationService) CreateOrderDelegation(ctx context.Context, payload dto.CreateOrderDelegationDTO) (int, error) {
	newID, err := s.orderDelegationRepository.CreateOrderDelegation(ctx, payload)
	if err != nil {
		s.logger.Error("Ошибка создания делегирования в сервисе", zap.Error(err), zap.Any("payload", payload))
		return 0, err
	}
	s.logger.Info("Делегирование успешно создано", zap.Int("newID", newID))
	return newID, nil
}

func (s *OrderDelegationService) DeleteOrderDelegation(ctx context.Context, id uint64) error {
	err := s.orderDelegationRepository.DeleteOrderDelegation(ctx, id)
	if err != nil {
		s.logger.Error("Ошибка удаления делегирования в сервисе", zap.Error(err), zap.Uint64("id", id))
		return err
	}
	s.logger.Info("Делегирование успешно удалено", zap.Uint64("id", id))
	return nil
}
*/