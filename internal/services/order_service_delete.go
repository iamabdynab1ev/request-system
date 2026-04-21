package services

import (
	"context"

	"request-system/internal/authz"
	apperrors "request-system/pkg/errors"
)

func (s *OrderService) DeleteOrder(ctx context.Context, orderID uint64) error {
	authCtx, err := s.buildAuthzContext(ctx, orderID)
	if err != nil {
		return err
	}
	if !authz.CanDo(authz.OrdersDelete, *authCtx) {
		return apperrors.ErrForbidden
	}
	if err := s.orderRepo.DeleteOrder(ctx, orderID); err != nil {
		return err
	}
	s.invalidateDashboardCache(ctx, true, true)
	return nil
}
