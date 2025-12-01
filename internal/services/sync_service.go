// Файл: internal/services/sync_service.go
package services

import (
	"context"
	"fmt"

	"go.uber.org/zap"

	"request-system/internal/dto" 
	"request-system/internal/sync"
)

// webhookContextKey - ключ для хранения логгера в контексте фоновых задач.
type webhookContextKey struct{}

// NewWebhookContext создает новый контекст с логгером для фоновой задачи.
func NewWebhookContext(logger *zap.Logger) context.Context {
	return context.WithValue(context.Background(), webhookContextKey{}, logger)
}

// LoggerFromContext извлекает логгер из контекста.
func LoggerFromContext(ctx context.Context) *zap.Logger {
	if logger, ok := ctx.Value(webhookContextKey{}).(*zap.Logger); ok {
		return logger
	}
	// Возвращаем глобальный логгер, если в контексте ничего нет (как запасной вариант)
	return zap.L()
}

// SyncServiceInterface определяет операции синхронизации.
// Теперь у нас только один метод, который работает с 1С.
type SyncServiceInterface interface {
	// Тип payload теперь dto.Webhook1CPayloadDTO
	Process1CReferences(ctx context.Context, payload dto.Webhook1CPayloadDTO) error
}

type SyncService struct {
	handler sync.HandlerInterface
	logger  *zap.Logger
}

func NewSyncService(handler sync.HandlerInterface, logger *zap.Logger) SyncServiceInterface {
	return &SyncService{
		handler: handler,
		logger:  logger,
	}
}

// Process1CReferences - единственный метод, который принимает данные от 1С и последовательно их обрабатывает.
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

	// Этап 1: Обработка независимых справочников
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

	// Этап 2: Обработка справочников, зависящих от Этапа 1
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

	// Этап 3: Обработка должностей
	if len(payload.Positions) > 0 {
		logger.Debug("Обработка должностей...")
		if err := s.handler.ProcessPositions(ctx, payload.Positions); err != nil {
			return fmt.Errorf("ошибка обработки должностей от 1С: %w", err)
		}
	}

	// Этап 4: Обработка пользователей
	if len(payload.Users) > 0 {
		logger.Debug("Обработка пользователей...")
		if err := s.handler.ProcessUsers(ctx, payload.Users); err != nil {
			return fmt.Errorf("ошибка обработки пользователей от 1С: %w", err)
		}
	}

	logger.Info("Фоновая обработка данных от 1С успешно завершена.")
	return nil
}
