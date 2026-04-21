package services

import (
	"context"
	"errors"
	"mime/multipart"
	"time"

	"github.com/google/uuid"
	"github.com/jackc/pgx/v5"

	"request-system/internal/authz"
	"request-system/internal/dto"
	apperrors "request-system/pkg/errors"
	"request-system/pkg/utils"
)

func (s *OrderService) UpdateOrder(ctx context.Context, orderID uint64, updateDTO dto.UpdateOrderDTO, file *multipart.FileHeader, explicitFields map[string]interface{}) (*dto.OrderResponseDTO, error) {
	currentOrder, err := s.orderRepo.FindByID(ctx, orderID)
	if err != nil {
		return nil, err
	}

	var (
		invalidateSummary  bool
		invalidateActivity bool
	)

	status, _ := s.statusRepo.FindStatus(ctx, currentOrder.StatusID)
	if status != nil && status.Code != nil && *status.Code == "CLOSED" {
		return nil, apperrors.NewBadRequestError("Заявка закрыта. Редактирование запрещено.")
	}

	authCtx, err := s.buildAuthzContextWithTarget(ctx, currentOrder)
	if err != nil {
		return nil, err
	}
	if !authz.CanDo(authz.OrdersUpdate, *authCtx) {
		return nil, apperrors.ErrForbidden
	}
	if err := s.validateUpdateFieldPermissions(authCtx, explicitFields, file); err != nil {
		return nil, err
	}
	if err := s.validateUpdateCommentRequirement(ctx, currentOrder, updateDTO); err != nil {
		return nil, err
	}

	if len(explicitFields) == 0 && file == nil {
		return nil, apperrors.NewBadRequestError("Нет данных для обновления.")
	}

	err = s.txManager.RunInTransaction(ctx, func(tx pgx.Tx) error {
		txID := uuid.New()
		now := time.Now().In(time.Local)

		updated := *currentOrder

		fieldsChanged := utils.SmartUpdate(&updated, explicitFields)
		updated.UpdatedAt = now

		routingChanged, err := s.applyUpdateExecutorRouting(ctx, tx, orderID, currentOrder, &updated, updateDTO, explicitFields, authCtx)
		if err != nil {
			return err
		}
		fieldsChanged = fieldsChanged || routingChanged

		s.calculateMetrics(ctx, &updated, currentOrder, updateDTO, authCtx.Actor.ID, now)

		metricsChanged := s.detectPersistedMetricChanges(orderID, currentOrder, &updated)
		if metricsChanged {
			fieldsChanged = true
		}

		historyChanged, err := s.detectAndLogChanges(ctx, tx, currentOrder, &updated, updateDTO, authCtx.Actor, txID, now)
		if err != nil {
			return err
		}

		if file != nil {
			if _, err := s.attachFileToOrderInTx(ctx, tx, orderID, authCtx.Actor.ID, file, &txID, &updated); err != nil {
				return err
			}
			fieldsChanged = true
		}

		invalidateSummary = dashboardSummaryAffected(currentOrder, &updated)
		invalidateActivity = historyChanged || file != nil

		if !fieldsChanged && !historyChanged {
			return apperrors.ErrNoChanges
		}

		return s.orderRepo.Update(ctx, tx, &updated)
	})
	if err != nil {
		if errors.Is(err, apperrors.ErrNoChanges) {
			return s.FindOrderByID(ctx, orderID)
		}
		return nil, err
	}

	s.invalidateDashboardCache(ctx, invalidateSummary, invalidateActivity)
	return s.FindOrderByID(ctx, orderID)
}
