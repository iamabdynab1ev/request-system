package services

import (
	"context"
	"errors"
	"fmt"
	"request-system/internal/dto"
	"request-system/internal/repositories"
	"request-system/pkg/contextkeys"
	apperrors "request-system/pkg/errors"
	"request-system/pkg/utils"

	"github.com/jackc/pgx/v5/pgxpool"
	"go.uber.org/zap"
)

type OrderServiceInterface interface {
	GetOrders(ctx context.Context, limit uint64, offset uint64) ([]dto.OrderDTO, uint64, error)
	FindOrder(ctx context.Context, id uint64) (*dto.OrderDTO, error)
	SoftDeleteOrder(ctx context.Context, id uint64) error
	CreateOrder(ctx context.Context, orderData dto.CreateOrderDTO) (int, error)
	UpdateOrder(ctx context.Context, id uint64, orderData dto.UpdateOrderDTO) error
}

type OrderService struct {
	pool           *pgxpool.Pool
	orderRepo      repositories.OrderRepositoryInterface
	commentRepo    repositories.OrderCommentRepositoryInterface
	delegationRepo repositories.OrderDelegationRepositoryInterface
	logger         *zap.Logger
	userRepo       repositories.UserRepositoryInterface
	statusRepo     repositories.StatusRepositoryInterface
	priorityRepo   repositories.ProretyRepositoryInterface
}

func NewOrderService(
	pool *pgxpool.Pool,
	orderRepo repositories.OrderRepositoryInterface,
	commentRepo repositories.OrderCommentRepositoryInterface,
	delegationRepo repositories.OrderDelegationRepositoryInterface,
	logger *zap.Logger,
	userRepo repositories.UserRepositoryInterface,
	statusRepo repositories.StatusRepositoryInterface,
	priorityRepo repositories.ProretyRepositoryInterface,
) OrderServiceInterface {
	return &OrderService{
		pool:           pool,
		orderRepo:      orderRepo,
		commentRepo:    commentRepo,
		delegationRepo: delegationRepo,
		logger:         logger,
		userRepo:       userRepo,
		statusRepo:     statusRepo,
		priorityRepo:   priorityRepo,
	}
}

func (s *OrderService) CreateOrder(ctx context.Context, orderData dto.CreateOrderDTO) (newOrderID int, err error) {
	creatorUserID, ok := ctx.Value(contextkeys.UserIDKey).(int)
	if !ok || creatorUserID == 0 {
		return 0, apperrors.ErrInvalidUserID
	}

	if orderData.StatusID == 0 {
		statusOpen, err := s.statusRepo.FindByCode(ctx, "OPEN")
		if err != nil {
			s.logger.Error("статус по умолчанию 'OPEN' не найден", zap.Error(err))
			return 0, fmt.Errorf("ошибка конфигурации: статус 'Открыто' не найден")
		}
		orderData.StatusID = statusOpen.ID
	}

	if orderData.ProretyID == 0 {
		priorityMedium, err := s.priorityRepo.FindByCode(ctx, "MEDIUM")
		if err != nil {
			s.logger.Error("приоритет по умолчанию 'MEDIUM' не найден", zap.Error(err))
			return 0, fmt.Errorf("ошибка конфигурации: приоритет 'Средний' не найден")
		}
		orderData.ProretyID = priorityMedium.Id
	}

	departmentHead, err := s.userRepo.FindHeadByDepartmentID(ctx, orderData.DepartmentID)
	if err != nil {
		s.logger.Warn("не удалось найти руководителя для департамента",
			zap.Int("departmentId", orderData.DepartmentID), zap.Error(err))
		if errors.Is(err, utils.ErrorNotFound) {
			return 0, apperrors.NewInvalidInputError("Невозможно создать заявку: для указанного подразделения не назначен руководитель.")
		}
		return 0, fmt.Errorf("внутренняя ошибка при поиске руководителя: %w", err)
	}

	tx, err := s.pool.Begin(ctx)
	if err != nil {
		s.logger.Error("Не удалось начать транзакцию", zap.Error(err))
		return 0, fmt.Errorf("ошибка начала транзакции: %w", err)
	}
	defer tx.Rollback(ctx)

	newOrderID, err = s.orderRepo.CreateOrderInTx(ctx, tx, creatorUserID, orderData, int(departmentHead.ID))

	if err != nil {
		s.logger.Error("Ошибка в orderRepo.CreateOrderInTx", zap.Error(err))
		return 0, err
	}

	delegationDto := dto.CreateOrderDelegationDTO{
		OrderID:         newOrderID,
		DelegatedUserID: int(departmentHead.ID),
		StatusID:        orderData.StatusID,
	}
	if err = s.delegationRepo.CreateOrderDelegationInTx(ctx, tx, creatorUserID, delegationDto); err != nil {
		s.logger.Error("Ошибка в delegationRepo.CreateOrderDelegationInTx", zap.Error(err), zap.Int("orderId", newOrderID))
		return 0, err
	}

	if orderData.Message != "" {
		commentDto := dto.CreateOrderCommentDTO{
			OrderID:  newOrderID,
			Message:  orderData.Message,
			StatusID: orderData.StatusID,
		}
		if err = s.commentRepo.CreateOrderCommentInTx(ctx, tx, creatorUserID, commentDto); err != nil {
			s.logger.Error("Ошибка в commentRepo.CreateOrderCommentInTx", zap.Error(err), zap.Int("orderId", newOrderID))
			return 0, err
		}
	}

	if err = tx.Commit(ctx); err != nil {
		s.logger.Error("Ошибка при коммите транзакции", zap.Error(err))
		return 0, fmt.Errorf("ошибка коммита транзакции: %w", err)
	}

	return newOrderID, nil
}

func (s *OrderService) UpdateOrder(ctx context.Context, id uint64, updateData dto.UpdateOrderDTO) (err error) {
	updatorUserID, ok := ctx.Value(contextkeys.UserIDKey).(int)
	if !ok || updatorUserID == 0 {
		return apperrors.ErrInvalidUserID
	}

	tx, err := s.pool.Begin(ctx)
	if err != nil {
		return fmt.Errorf("ошибка начала транзакции: %w", err)
	}
	defer func() {
		if err != nil {
			_ = tx.Rollback(context.Background())
		}
	}()

	orderState, err := s.orderRepo.FindOrderForUpdateInTx(ctx, tx, id)
	if err != nil {
		return err
	}

	if err = s.orderRepo.UpdateOrderInTx(ctx, tx, id, updateData); err != nil {
		return err
	}

	if updateData.ExecutorID != 0 && int(orderState.CurrentExecutorID.Int32) != updateData.ExecutorID {
		delegationDto := dto.CreateOrderDelegationDTO{
			OrderID:         int(id),
			DelegatedUserID: updateData.ExecutorID,
			StatusID:        orderState.CurrentStatusID,
		}
		if updateData.StatusID != 0 {
			delegationDto.StatusID = updateData.StatusID
		}
		if err = s.delegationRepo.CreateOrderDelegationInTx(ctx, tx, updatorUserID, delegationDto); err != nil {
			return err
		}
	}

	if updateData.StatusID != 0 && orderState.CurrentStatusID != updateData.StatusID {
		commentMessage := fmt.Sprintf("Статус заявки изменен (старый ID: %d, новый ID: %d)", orderState.CurrentStatusID, updateData.StatusID)
		commentDto := dto.CreateOrderCommentDTO{
			OrderID:  int(id),
			Message:  commentMessage,
			StatusID: updateData.StatusID,
		}
		if err = s.commentRepo.CreateOrderCommentInTx(ctx, tx, updatorUserID, commentDto); err != nil {
			return err
		}
	}

	return tx.Commit(ctx)
}

func (s *OrderService) GetOrders(ctx context.Context, limit uint64, offset uint64) ([]dto.OrderDTO, uint64, error) {
	return s.orderRepo.GetOrders(ctx, limit, offset)
}

func (s *OrderService) FindOrder(ctx context.Context, id uint64) (*dto.OrderDTO, error) {
	return s.orderRepo.FindOrder(ctx, id)
}

func (s *OrderService) SoftDeleteOrder(ctx context.Context, id uint64) error {
	return s.orderRepo.SoftDeleteOrder(ctx, id)
}
