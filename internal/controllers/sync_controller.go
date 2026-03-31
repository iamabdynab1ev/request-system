package controllers

import (
	"net/http"

	"request-system/internal/dto"
	"request-system/internal/services"
	apperrors "request-system/pkg/errors"
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

		apiErr := apperrors.NewHttpError(http.StatusBadRequest, "Неверный формат JSON", err, nil)
		return utils.ErrorResponse(ctx, apiErr, c.logger)
	}

	accepted, err := c.syncService.Enqueue1CReferences(ctx.Request().Context(), payload)
	if err != nil {
		c.logger.Error("Не удалось поставить синхронизацию 1С в обработку", zap.Error(err))
		return utils.ErrorResponse(ctx, err, c.logger)
	}

	message := "Запрос принят в обработку"
	if !accepted {
		message = "Синхронизация 1С уже выполняется"
	}

	return utils.SuccessResponse(ctx, nil, message, http.StatusAccepted)
}

func (c *SyncController) HandleSyncAll(ctx echo.Context) error {
	c.logger.Warn("Получен запрос на устаревший эндпоинт /sync/run. Этот метод больше не выполняет никаких действий.")

	apiErr := apperrors.NewHttpError(
		http.StatusNotImplemented,
		"Этот метод синхронизации больше не поддерживается. Перейдите на использование вебхуков 1С.",
		nil,
		nil,
	)
	return utils.ErrorResponse(ctx, apiErr, c.logger)
}
