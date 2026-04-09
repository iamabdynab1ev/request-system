package v1c

import (
	"net/http"

	"request-system/internal/dto"
	"request-system/internal/services"
	"request-system/pkg/utils"

	"github.com/labstack/echo/v4"
	"go.uber.org/zap"
)

type Controller struct {
	syncService services.SyncServiceInterface
	logger      *zap.Logger
}

func NewController(service services.SyncServiceInterface, logger *zap.Logger) *Controller {
	return &Controller{
		syncService: service,
		logger:      logger.Named("1c_webhook_controller"),
	}
}

func (c *Controller) HandleReferencesWebhook(ctx echo.Context) error {
	var payload dto.Webhook1CPayloadDTO

	if err := ctx.Bind(&payload); err != nil {
		c.logger.Warn("Не удалось распознать тело запроса от 1С", zap.Error(err))
		return utils.ErrorResponse(ctx, err, c.logger)
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
