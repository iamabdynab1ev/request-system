package services

import (
	"context"

	sq "github.com/Masterminds/squirrel"
	"go.uber.org/zap"

	"request-system/internal/authz"
	"request-system/internal/dto"
	"request-system/internal/entities"
	apperrors "request-system/pkg/errors"
	"request-system/pkg/types"
	"request-system/pkg/utils"
)

func (s *OrderService) GetOrders(ctx context.Context, filter types.Filter, onlyCreated bool, onlyAssigned bool, onlyInvolved bool) (*dto.OrderListResponseDTO, error) {
	userID, err := utils.GetUserIDFromCtx(ctx)
	if err != nil {
		return nil, apperrors.ErrUserNotFound
	}

	permissionsMap, err := s.resolvePermissionsMap(ctx, userID)
	if err != nil {
		return nil, apperrors.ErrUnauthorized
	}

	actor, err := s.resolveActorFromContext(ctx, userID)
	if err != nil {
		return nil, apperrors.ErrUserNotFound
	}

	authCtx := authz.Context{Actor: actor, Permissions: permissionsMap}
	if !authz.CanDo(authz.OrdersView, authCtx) {
		s.logger.Warn("Попытка доступа без прав на просмотр заявок", zap.Uint64("user_id", userID))
		return nil, apperrors.ErrForbidden
	}

	securityBuilder := sq.And{}

	if !authCtx.HasPermission(authz.ScopeAll) && !authCtx.HasPermission(authz.ScopeAllView) {
		scopeConditions := sq.Or{}

		if authCtx.HasPermission(authz.ScopeDepartment) && actor.DepartmentID != nil {
			scopeConditions = append(scopeConditions, sq.Eq{"o.department_id": *actor.DepartmentID})
		}
		if authCtx.HasPermission(authz.ScopeBranch) && actor.BranchID != nil {
			scopeConditions = append(scopeConditions, sq.Eq{"o.branch_id": *actor.BranchID})
		}
		if authCtx.HasPermission(authz.ScopeOtdel) && actor.OtdelID != nil {
			scopeConditions = append(scopeConditions, sq.Eq{"o.otdel_id": *actor.OtdelID})
		}
		if authCtx.HasPermission(authz.ScopeOffice) && actor.OfficeID != nil {
			scopeConditions = append(scopeConditions, sq.Eq{"o.office_id": *actor.OfficeID})
		}
		if authCtx.HasPermission(authz.ScopeOwn) {
			scopeConditions = append(scopeConditions, sq.Eq{"o.user_id": actor.ID})
			scopeConditions = append(scopeConditions, sq.Eq{"o.executor_id": actor.ID})
			scopeConditions = append(scopeConditions, sq.Expr("o.id IN (SELECT DISTINCT order_id FROM order_history WHERE user_id = ?)", actor.ID))
		}

		if len(scopeConditions) == 0 {
			return &dto.OrderListResponseDTO{List: []dto.OrderResponseDTO{}, TotalCount: 0}, nil
		}

		securityBuilder = append(securityBuilder, scopeConditions)
	}

	if onlyCreated {
		securityBuilder = append(securityBuilder, sq.Eq{"o.user_id": actor.ID})
	}
	if onlyAssigned {
		securityBuilder = append(securityBuilder, sq.Eq{"o.executor_id": actor.ID})
	}
	if onlyInvolved {
		securityBuilder = append(
			securityBuilder,
			sq.Expr(
				`o.id IN (SELECT DISTINCT order_id FROM order_history WHERE user_id = ?)
				 AND o.user_id <> ?
				 AND (o.executor_id IS NULL OR o.executor_id <> ?)`,
				actor.ID,
				actor.ID,
				actor.ID,
			),
		)
	}

	orders, totalCount, err := s.orderRepo.GetOrders(ctx, filter, securityBuilder)
	if err != nil {
		return nil, err
	}
	if len(orders) == 0 {
		return &dto.OrderListResponseDTO{List: []dto.OrderResponseDTO{}, TotalCount: 0}, nil
	}

	dtos := s.mapOrdersToDTOs(ctx, orders, filter.IncludeAttachments)
	return &dto.OrderListResponseDTO{List: dtos, TotalCount: totalCount}, nil
}

func (s *OrderService) FindOrderByID(ctx context.Context, orderID uint64) (*dto.OrderResponseDTO, error) {
	authCtx, err := s.buildAuthzContext(ctx, orderID)
	if err != nil {
		return nil, err
	}
	if !authz.CanDo(authz.OrdersView, *authCtx) {
		return nil, apperrors.ErrForbidden
	}

	order := authCtx.Target.(*entities.Order)
	attachments := s.loadOrderAttachments(ctx, order.ID, 100, 0)
	return s.toResponseDTO(order, nil, nil, attachments), nil
}

func (s *OrderService) FindOrderByIDForTelegram(ctx context.Context, userID uint64, orderID uint64) (*entities.Order, error) {
	if userID == 0 {
		s.logger.Error("FindOrderByIDForTelegram вызван с userID=0", zap.Uint64("order_id", orderID), zap.Stack("stacktrace"))
		return nil, apperrors.ErrUserNotFound
	}

	if orderID == 0 {
		s.logger.Error("FindOrderByIDForTelegram вызван с orderID=0", zap.Uint64("user_id", userID))
		return nil, apperrors.NewBadRequestError("ID заявки не указан.")
	}

	order, err := s.orderRepo.FindByID(ctx, orderID)
	if err != nil {
		s.logger.Warn("Заявка не найдена", zap.Uint64("order_id", orderID), zap.Uint64("user_id", userID), zap.Error(err))
		return nil, err
	}

	permissionsMap, err := s.resolvePermissionsMap(ctx, userID)
	if err != nil {
		return nil, apperrors.ErrUnauthorized
	}

	user, err := s.resolveActorFromContext(ctx, userID)
	if err != nil {
		s.logger.Error("Пользователь не найден при проверке прав через Telegram", zap.Uint64("user_id", userID), zap.Uint64("order_id", orderID), zap.Error(err))
		return nil, apperrors.ErrUserNotFound
	}

	authCtx := authz.Context{Actor: user, Permissions: permissionsMap, Target: order}
	if !authz.CanDo(authz.OrdersView, authCtx) {
		s.logger.Warn("Попытка доступа к заявке без прав через Telegram", zap.Uint64("user_id", userID), zap.Uint64("order_id", orderID), zap.String("user_fio", user.Fio))
		return nil, apperrors.ErrForbidden
	}

	return order, nil
}
