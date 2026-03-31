package services

import (
	"context"
	"fmt"
	"sync/atomic"
	"time"

	"go.uber.org/zap"

	"request-system/internal/dto"
	"request-system/internal/sync"
)

type webhookContextKey struct{}

func NewWebhookContext(logger *zap.Logger) context.Context {
	return context.WithValue(context.Background(), webhookContextKey{}, logger)
}

func LoggerFromContext(ctx context.Context) *zap.Logger {
	if logger, ok := ctx.Value(webhookContextKey{}).(*zap.Logger); ok {
		return logger
	}
	return zap.L()
}

type SyncServiceInterface interface {
	Enqueue1CReferences(ctx context.Context, payload dto.Webhook1CPayloadDTO) (bool, error)
	Process1CReferences(ctx context.Context, payload dto.Webhook1CPayloadDTO) error
}

type SyncService struct {
	handler sync.HandlerInterface
	logger  *zap.Logger
	running atomic.Bool
	runSeq  atomic.Uint64
}

func NewSyncService(handler sync.HandlerInterface, logger *zap.Logger) SyncServiceInterface {
	return &SyncService{
		handler: handler,
		logger:  logger.Named("sync_1c"),
	}
}

func (s *SyncService) Enqueue1CReferences(ctx context.Context, payload dto.Webhook1CPayloadDTO) (bool, error) {
	_ = ctx

	if !s.running.CompareAndSwap(false, true) {
		s.logger.Warn("Синхронизация 1С уже выполняется, новый запуск отклонен", syncPayloadFields(payload)...)
		return false, nil
	}

	runID := s.runSeq.Add(1)
	fields := append([]zap.Field{zap.Uint64("run_id", runID)}, syncPayloadFields(payload)...)
	s.logger.Info("Запуск синхронизации 1С принят в обработку", fields...)

	go s.run1CReferences(runID, payload)
	return true, nil
}

func (s *SyncService) run1CReferences(runID uint64, payload dto.Webhook1CPayloadDTO) {
	startedAt := time.Now()
	logger := s.logger.With(append([]zap.Field{zap.Uint64("run_id", runID)}, syncPayloadFields(payload)...)...)
	bgCtx := NewWebhookContext(logger)
	defer s.running.Store(false)

	logger.Info("Фоновая синхронизация 1С запущена")

	if err := s.Process1CReferences(bgCtx, payload); err != nil {
		logger.Error(
			"Фоновая синхронизация 1С завершилась с ошибкой",
			zap.Duration("duration", time.Since(startedAt)),
			zap.Error(err),
		)
		return
	}

	logger.Info("Фоновая синхронизация 1С завершена успешно", zap.Duration("duration", time.Since(startedAt)))
}

func (s *SyncService) Process1CReferences(ctx context.Context, payload dto.Webhook1CPayloadDTO) error {
	logger := LoggerFromContext(ctx)
	logger.Info("Начало фоновой обработки данных, полученных от 1С",
		zap.Int("departments", len(payload.Departments)),
		zap.Int("otdels", len(payload.Otdels)),
		zap.Int("branches", len(payload.Branches)),
		zap.Int("offices", len(payload.Offices)),
		zap.Int("positions", len(payload.Positions)),
		zap.Int("users", len(payload.Users)),
	)

	if len(payload.Departments) > 0 {
		logger.Debug("Обработка департаментов...")
		if err := s.handler.ProcessDepartments(ctx, payload.Departments); err != nil {
			return fmt.Errorf("ошибка обработки департаментов от 1С: %w", err)
		}
	}
	if len(payload.Branches) > 0 {
		logger.Debug("Обработка филиалов...")
		if err := s.handler.ProcessBranches(ctx, payload.Branches); err != nil {
			return fmt.Errorf("ошибка обработки филиалов от 1С: %w", err)
		}
	}
	if len(payload.Otdels) > 0 {
		logger.Debug("Обработка отделов...")
		if err := s.handler.ProcessOtdels(ctx, payload.Otdels); err != nil {
			return fmt.Errorf("ошибка обработки отделов от 1С: %w", err)
		}
	}
	if len(payload.Offices) > 0 {
		logger.Debug("Обработка офисов...")
		if err := s.handler.ProcessOffices(ctx, payload.Offices); err != nil {
			return fmt.Errorf("ошибка обработки офисов от 1С: %w", err)
		}
	}
	if len(payload.Positions) > 0 {
		logger.Debug("Обработка должностей...")
		if err := s.handler.ProcessPositions(ctx, payload.Positions); err != nil {
			return fmt.Errorf("ошибка обработки должностей от 1С: %w", err)
		}
	}
	if len(payload.Users) > 0 {
		logger.Debug("Обработка пользователей...")
		if err := s.handler.ProcessUsers(ctx, payload.Users); err != nil {
			return fmt.Errorf("ошибка обработки пользователей от 1С: %w", err)
		}
	}

	logger.Info("Фоновая обработка данных от 1С успешно завершена")
	return nil
}

func syncPayloadFields(payload dto.Webhook1CPayloadDTO) []zap.Field {
	return []zap.Field{
		zap.Int("departments", len(payload.Departments)),
		zap.Int("otdels", len(payload.Otdels)),
		zap.Int("branches", len(payload.Branches)),
		zap.Int("offices", len(payload.Offices)),
		zap.Int("positions", len(payload.Positions)),
		zap.Int("users", len(payload.Users)),
	}
}
