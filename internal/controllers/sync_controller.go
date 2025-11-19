// Файл: internal/controllers/sync_controller.go
package controllers

import (
	"net/http"

	"request-system/internal/dto"
	"request-system/internal/services"
	apperrors "request-system/pkg/errors" // <-- ДОБАВЛЯЕМ ИМПОРТ APP ERRORS
	"request-system/pkg/utils"

	"github.com/labstack/echo/v4"
	"go.uber.org/zap"
)

type SyncController struct {
	syncService services.SyncServiceInterface
	logger      *zap.Logger
}

func NewSyncController(service services.SyncServiceInterface, logger *zap.Logger) *SyncController {
	return &SyncController{
		syncService: service,
		logger:      logger.Named("sync_controller"),
	}
}

func (c *SyncController) HandleSyncFrom1C(ctx echo.Context) error {
	var payload dto.Webhook1CPayloadDTO

	if err := ctx.Bind(&payload); err != nil {
		c.logger.Warn("Не удалось распознать тело запроса вебхука от 1С. Проверьте структуру JSON.", zap.Error(err))

		// ИСПРАВЛЕНИЕ: Создаем объект ошибки и передаем его
		apiErr := apperrors.NewHttpError(http.StatusBadRequest, "Неверный формат JSON", err, nil)
		return utils.ErrorResponse(ctx, apiErr, c.logger)
	}

	go func() {
		bgCtx := services.NewWebhookContext(c.logger)
		if err := c.syncService.Process1CReferences(bgCtx, payload); err != nil {
			c.logger.Error("Фоновая обработка данных от 1С завершилась с ошибкой", zap.Error(err))
		}
	}()

	return utils.SuccessResponse(ctx, nil, "Запрос принят в обработку", http.StatusAccepted)
}

func (c *SyncController) HandleSyncAll(ctx echo.Context) error {
	c.logger.Warn("Получен запрос на устаревший эндпоинт /sync/run. Этот метод больше не выполняет никаких действий.")

	// ИСПРАВЛЕНИЕ: Создаем объект ошибки и передаем его
	apiErr := apperrors.NewHttpError(
		http.StatusNotImplemented,
		"Этот метод синхронизации больше не поддерживается. Перейдите на использование вебхуков 1С.",
		nil,
		nil,
	)
	return utils.ErrorResponse(ctx, apiErr, c.logger)
}
