// Файл: internal/integrations/1c/controller.go
package v1c

import (
	"net/http"

	"request-system/internal/dto"
	"request-system/internal/services"

	"request-system/pkg/utils"

	"github.com/labstack/echo/v4"
	"go.uber.org/zap"
)

// Controller обрабатывает входящие вебхук-запросы от 1С.
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

// HandleReferencesWebhook принимает и обрабатывает данные от 1С.
func (c *Controller) HandleReferencesWebhook(ctx echo.Context) error {
	var payload dto.Webhook1CPayloadDTO

	if err := ctx.Bind(&payload); err != nil {
		c.logger.Warn("Не удалось распознать тело запроса от 1С", zap.Error(err))
		return utils.ErrorResponse(ctx, err, c.logger)
	}

	go func() {
		bgCtx := services.NewWebhookContext(c.logger)
		if err := c.syncService.Process1CReferences(bgCtx, payload); err != nil {
			c.logger.Error("Фоновая обработка данных от 1С завершилась с ошибкой", zap.Error(err))
		}
	}()

	return utils.SuccessResponse(ctx, nil, "Запрос принят в обработку", http.StatusAccepted)
}
