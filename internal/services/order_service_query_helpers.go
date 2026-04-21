package services

import (
	"context"
	"time"

	"go.uber.org/zap"

	"request-system/internal/authz"
	"request-system/internal/dto"
	"request-system/internal/entities"
	apperrors "request-system/pkg/errors"
	"request-system/pkg/types"
	"request-system/pkg/utils"
)

func (s *OrderService) GetStatusByID(ctx context.Context, id uint64) (*entities.Status, error) {
	return s.statusRepo.FindStatus(ctx, id)
}

func (s *OrderService) GetPriorityByID(ctx context.Context, id uint64) (*entities.Priority, error) {
	return s.priorityRepo.FindByID(ctx, id)
}

func (s *OrderService) GetUserStats(ctx context.Context, userID uint64) (*types.UserOrderStats, error) {
	return s.orderRepo.GetUserOrderStats(ctx, userID, time.Now().AddDate(0, 0, -30))
}

func (s *OrderService) mapOrdersToDTOs(ctx context.Context, orders []entities.Order, includeAttachments bool) []dto.OrderResponseDTO {
	attachMap := make(map[uint64][]entities.Attachment)

	if includeAttachments {
		orderIDs := make([]uint64, len(orders))
		for i, o := range orders {
			orderIDs[i] = o.ID
		}

		if found, err := s.attachRepo.FindAttachmentsByOrderIDs(ctx, orderIDs); err == nil {
			attachMap = found
		} else {
			s.logger.Warn("Не удалось загрузить вложения для списка заявок", zap.Int("orders_count", len(orderIDs)), zap.Error(err))
		}
	}

	res := make([]dto.OrderResponseDTO, len(orders))
	for i, o := range orders {
		atts := attachMap[o.ID]
		res[i] = *s.toResponseDTO(&o, nil, nil, atts)
	}
	return res
}

func (s *OrderService) loadOrderAttachments(ctx context.Context, orderID uint64, limit, offset int) []entities.Attachment {
	attachments, err := s.attachRepo.FindAllByOrderID(ctx, orderID, limit, offset)
	if err == nil {
		return attachments
	}

	s.logger.Warn("Не удалось загрузить вложения заявки", zap.Uint64("order_id", orderID), zap.Error(err))
	return nil
}

func (s *OrderService) toResponseDTO(o *entities.Order, cr, ex *entities.User, atts []entities.Attachment) *dto.OrderResponseDTO {
	d := &dto.OrderResponseDTO{
		ID:                       o.ID,
		Name:                     o.Name,
		StatusID:                 o.StatusID,
		CreatedAt:                o.CreatedAt.Format(time.RFC3339),
		UpdatedAt:                o.UpdatedAt.Format(time.RFC3339),
		OrderTypeID:              o.OrderTypeID,
		Address:                  o.Address,
		DepartmentID:             o.DepartmentID,
		OtdelID:                  o.OtdelID,
		BranchID:                 o.BranchID,
		OfficeID:                 o.OfficeID,
		EquipmentID:              o.EquipmentID,
		EquipmentTypeID:          o.EquipmentTypeID,
		PriorityID:               o.PriorityID,
		Duration:                 o.Duration,
		CompletedAt:              o.CompletedAt,
		ResolutionTimeSeconds:    o.ResolutionTimeSeconds,
		FirstResponseTimeSeconds: o.FirstResponseTimeSeconds,
		CreatorID:                o.CreatorID,
		CreatorName:              o.CreatorName,
	}

	if o.ExecutorID != nil {
		d.ExecutorID = o.ExecutorID
		d.ExecutorName = o.ExecutorName
	}

	if o.ResolutionTimeSeconds != nil {
		d.ResolutionTimeFormatted = utils.FormatSecondsToHumanReadable(*o.ResolutionTimeSeconds)
	}
	if o.FirstResponseTimeSeconds != nil {
		d.FirstResponseTimeFormatted = utils.FormatSecondsToHumanReadable(*o.FirstResponseTimeSeconds)
	}

	d.Attachments = make([]dto.AttachmentResponseDTO, len(atts))
	for i, a := range atts {
		d.Attachments[i] = dto.AttachmentResponseDTO{ID: a.ID, FileName: a.FileName, URL: "/uploads/" + a.FilePath}
	}
	return d
}

func (s *OrderService) buildAuthzContext(ctx context.Context, orderID uint64) (*authz.Context, error) {
	if orderID == 0 {
		userID, err := utils.GetUserIDFromCtx(ctx)
		if err != nil {
			return nil, apperrors.ErrUnauthorized
		}
		permissionsMap, err := utils.GetPermissionsMapFromCtx(ctx)
		if err != nil {
			return nil, apperrors.ErrUnauthorized
		}
		actor, err := s.resolveActorFromContext(ctx, userID)
		if err != nil {
			return nil, apperrors.ErrUserNotFound
		}
		return &authz.Context{Actor: actor, Permissions: permissionsMap}, nil
	}

	target, err := s.orderRepo.FindByID(ctx, orderID)
	if err != nil {
		return nil, err
	}
	return s.buildAuthzContextWithTarget(ctx, target)
}

func (s *OrderService) buildAuthzContextWithTarget(ctx context.Context, target *entities.Order) (*authz.Context, error) {
	userID, err := utils.GetUserIDFromCtx(ctx)
	if err != nil {
		return nil, apperrors.ErrUnauthorized
	}
	permissionsMap, err := utils.GetPermissionsMapFromCtx(ctx)
	if err != nil {
		return nil, apperrors.ErrUnauthorized
	}
	actor, err := s.resolveActorFromContext(ctx, userID)
	if err != nil {
		return nil, apperrors.ErrUserNotFound
	}

	ctxAuth := &authz.Context{Actor: actor, Permissions: permissionsMap, Target: target}
	wasParticipant, _ := s.historyRepo.IsUserParticipant(ctx, target.ID, userID)
	ctxAuth.IsParticipant = (target.CreatorID == userID) || (target.ExecutorID != nil && *target.ExecutorID == userID) || wasParticipant
	return ctxAuth, nil
}
