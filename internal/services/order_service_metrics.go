package services

import (
	"context"
	"strings"
	"time"

	"go.uber.org/zap"

	"request-system/internal/dto"
	"request-system/internal/entities"
	pkgconstants "request-system/pkg/constants"
	"request-system/pkg/utils"
)

func (s *OrderService) detectPersistedMetricChanges(orderID uint64, currentOrder, updated *entities.Order) bool {
	metricsChanged := false

	if updated.FirstResponseTimeSeconds != nil {
		if currentOrder.FirstResponseTimeSeconds == nil {
			metricsChanged = true
			s.logger.Info("Новая метрика: first_response_time_seconds",
				zap.Uint64("order_id", orderID),
				zap.Uint64("value", *updated.FirstResponseTimeSeconds))
		} else if *updated.FirstResponseTimeSeconds != *currentOrder.FirstResponseTimeSeconds {
			metricsChanged = true
			s.logger.Info("Обновлена метрика: first_response_time_seconds",
				zap.Uint64("order_id", orderID),
				zap.Uint64("old", *currentOrder.FirstResponseTimeSeconds),
				zap.Uint64("new", *updated.FirstResponseTimeSeconds))
		}
	}

	if updated.ResolutionTimeSeconds != nil {
		if currentOrder.ResolutionTimeSeconds == nil {
			metricsChanged = true
			s.logger.Info("Новая метрика: resolution_time_seconds",
				zap.Uint64("order_id", orderID),
				zap.Uint64("value", *updated.ResolutionTimeSeconds))
		} else if *updated.ResolutionTimeSeconds != *currentOrder.ResolutionTimeSeconds {
			metricsChanged = true
			s.logger.Info("Обновлена метрика: resolution_time_seconds",
				zap.Uint64("order_id", orderID),
				zap.Uint64("old", *currentOrder.ResolutionTimeSeconds),
				zap.Uint64("new", *updated.ResolutionTimeSeconds))
		}
	}

	if updated.IsFirstContactResolution != nil {
		if currentOrder.IsFirstContactResolution == nil {
			metricsChanged = true
			s.logger.Info("Новая метрика: is_first_contact_resolution",
				zap.Uint64("order_id", orderID),
				zap.Bool("value", *updated.IsFirstContactResolution))
		} else if *updated.IsFirstContactResolution != *currentOrder.IsFirstContactResolution {
			metricsChanged = true
			s.logger.Info("Обновлена метрика: is_first_contact_resolution",
				zap.Uint64("order_id", orderID),
				zap.Bool("old", *currentOrder.IsFirstContactResolution),
				zap.Bool("new", *updated.IsFirstContactResolution))
		}
	}

	if !timePointersEqual(currentOrder.CompletedAt, updated.CompletedAt) {
		metricsChanged = true
		s.logger.Info("Обновлена дата завершения",
			zap.Uint64("order_id", orderID),
			zap.Any("old", currentOrder.CompletedAt),
			zap.Any("new", updated.CompletedAt))
	}

	if metricsChanged {
		s.logger.Info("Обнаружены изменения метрик, сохраняем заявку",
			zap.Uint64("order_id", orderID))
	}

	return metricsChanged
}

func (s *OrderService) calculateMetrics(ctx context.Context, newOrder, oldOrder *entities.Order, dto dto.UpdateOrderDTO, actorID uint64, now time.Time) {
	newStatus, _ := s.statusRepo.FindStatus(ctx, newOrder.StatusID)
	newCode := ""
	if newStatus != nil && newStatus.Code != nil {
		newCode = *newStatus.Code
	}
	oldStatus, _ := s.statusRepo.FindStatus(ctx, oldOrder.StatusID)
	oldCode := ""
	if oldStatus != nil && oldStatus.Code != nil {
		oldCode = *oldStatus.Code
	}

	loc := time.Local
	createdAt := oldOrder.CreatedAt.In(loc)
	nowAt := now.In(loc)

	diff := int64(nowAt.Sub(createdAt).Seconds())
	if diff < 0 {
		s.logger.Warn("Отрицательная разница времени",
			zap.Time("created", createdAt),
			zap.Time("now", nowAt),
			zap.Int64("diff", diff))
		diff = 0
	}
	seconds := uint64(diff)

	// Подробный лог нужен для разборов расчёта SLA и FCR на спорных сценариях.
	s.logger.Info("Расчёт метрик времени",
		zap.Uint64("order_id", newOrder.ID),
		zap.Time("created_at", createdAt),
		zap.Time("now", nowAt),
		zap.Uint64("diff_seconds", seconds),
		zap.Uint64("actor_id", actorID),
		zap.Uint64("creator_id", oldOrder.CreatorID),
		zap.String("old_status", oldCode),
		zap.String("new_status", newCode))

	newStatusResolved := isOrderResolvedStatus(newCode)
	oldStatusResolved := isOrderResolvedStatus(oldCode)

	if oldOrder.FirstResponseTimeSeconds == nil || *oldOrder.FirstResponseTimeSeconds == 0 {
		isExecutorAction := false

		if oldOrder.ExecutorID != nil && *oldOrder.ExecutorID == actorID {
			isExecutorAction = true
		}
		if newOrder.ExecutorID != nil && *newOrder.ExecutorID == actorID {
			isExecutorAction = true
		}

		hasComment := dto.Comment != nil && strings.TrimSpace(*dto.Comment) != ""
		statusChanged := newOrder.StatusID != oldOrder.StatusID
		executorChanged := (oldOrder.ExecutorID == nil && newOrder.ExecutorID != nil) ||
			(oldOrder.ExecutorID != nil && newOrder.ExecutorID != nil && *oldOrder.ExecutorID != *newOrder.ExecutorID)

		if isExecutorAction && (statusChanged || executorChanged || hasComment) {
			newOrder.FirstResponseTimeSeconds = &seconds

			isFCR := false
			if newStatusResolved && oldOrder.FirstResponseTimeSeconds == nil {
				isFCR = true
			}
			newOrder.IsFirstContactResolution = &isFCR

			s.logger.Info("Установлено время первого отклика",
				zap.Uint64("order_id", newOrder.ID),
				zap.Uint64("seconds", seconds),
				zap.Bool("is_fcr", isFCR),
				zap.Bool("is_executor_action", isExecutorAction))
		}
	}

	if newStatusResolved && !oldStatusResolved {
		s.applyResolutionMetrics(newOrder, now, seconds)
	}

	if oldStatusResolved && !newStatusResolved {
		s.resetResolutionMetrics(newOrder)
	}
}

func isOrderResolvedStatus(code string) bool {
	return code == pkgconstants.StatusCompleted || code == pkgconstants.StatusClosed
}

func (s *OrderService) applyResolutionMetrics(newOrder *entities.Order, now time.Time, resolutionSeconds uint64) {
	newOrder.ResolutionTimeSeconds = &resolutionSeconds
	completedAt := now.In(time.Local)
	newOrder.CompletedAt = &completedAt

	if newOrder.FirstResponseTimeSeconds == nil {
		isFCR := true
		newOrder.IsFirstContactResolution = &isFCR
	}

	s.logger.Info("Установлены метрики завершения",
		zap.Uint64("order_id", newOrder.ID),
		zap.Uint64("resolution_seconds", resolutionSeconds),
		zap.Time("completed_at", completedAt),
		zap.Any("is_first_contact_resolution", newOrder.IsFirstContactResolution))
}

func (s *OrderService) resetResolutionMetrics(newOrder *entities.Order) {
	s.logger.Info("Сброс метрик завершения при возврате заявки в работу",
		zap.Uint64("order_id", newOrder.ID))
	newOrder.CompletedAt = nil
	newOrder.ResolutionTimeSeconds = nil
	newOrder.IsFirstContactResolution = nil
}

func (s *OrderService) invalidateDashboardCache(ctx context.Context, invalidateSummary bool, invalidateActivity bool) {
	if s.cacheRepo == nil {
		return
	}
	if !invalidateSummary && !invalidateActivity {
		return
	}

	if invalidateSummary {
		if _, err := s.cacheRepo.Incr(ctx, pkgconstants.DashboardCacheVersionSummaryKey); err != nil {
			s.logger.Warn("Не удалось обновить summary-версию кеша дашборда", zap.Error(err))
		}
	}

	if invalidateActivity {
		if _, err := s.cacheRepo.Incr(ctx, pkgconstants.DashboardCacheVersionActivityKey); err != nil {
			s.logger.Warn("Не удалось обновить activity-версию кеша дашборда", zap.Error(err))
		}
	}
}

func dashboardSummaryAffected(old, updated *entities.Order) bool {
	if old == nil || updated == nil {
		return true
	}

	return old.StatusID != updated.StatusID ||
		utils.DiffPtr(old.PriorityID, updated.PriorityID) ||
		utils.DiffPtr(old.ExecutorID, updated.ExecutorID) ||
		utils.DiffPtr(old.DepartmentID, updated.DepartmentID) ||
		utils.DiffPtr(old.OtdelID, updated.OtdelID) ||
		utils.DiffPtr(old.BranchID, updated.BranchID) ||
		utils.DiffPtr(old.OfficeID, updated.OfficeID) ||
		utils.DiffPtr(old.OrderTypeID, updated.OrderTypeID) ||
		!utils.TimeEqual(old.Duration, updated.Duration) ||
		!timePointersEqual(old.CompletedAt, updated.CompletedAt) ||
		utils.DiffPtr(old.ResolutionTimeSeconds, updated.ResolutionTimeSeconds) ||
		utils.DiffPtr(old.FirstResponseTimeSeconds, updated.FirstResponseTimeSeconds) ||
		boolPointersDiffer(old.IsFirstContactResolution, updated.IsFirstContactResolution)
}

func boolPointersDiffer(old, updated *bool) bool {
	if old == nil || updated == nil {
		return old != updated
	}

	return *old != *updated
}
