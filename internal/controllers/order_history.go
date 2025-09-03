package controllers

import (
	"net/http"
	"strconv"

	"request-system/internal/services"
	apperrors "request-system/pkg/errors"
	"request-system/pkg/utils"

	"github.com/labstack/echo/v4"
	"go.uber.org/zap"
)

type OrderHistoryController struct {
	historyService services.OrderHistoryServiceInterface
	logger         *zap.Logger
}

func NewOrderHistoryController(service services.OrderHistoryServiceInterface, logger *zap.Logger) *OrderHistoryController {
	return &OrderHistoryController{historyService: service, logger: logger}
}

func (c *OrderHistoryController) GetHistoryForOrder(ctx echo.Context) error {
	reqCtx := utils.Ctx(ctx, 5)

	orderID, err := strconv.ParseUint(ctx.Param("orderID"), 10, 64)
	if err != nil {
		c.logger.Error("GetHistoryForOrder: неверный формат ID заявки", zap.String("orderID", ctx.Param("orderID")), zap.Error(err))
		return utils.ErrorResponse(
			ctx,
			apperrors.NewHttpError(
				http.StatusBadRequest,
				"Неверный формат ID заявки",
				err,
				map[string]interface{}{"param": ctx.Param("orderID")},
			),
			c.logger,
		)
	}

	timeline, err := c.historyService.GetTimelineByOrderID(reqCtx, orderID)
	if err != nil {
		c.logger.Error("GetHistoryForOrder: ошибка при получении истории заявки", zap.Uint64("orderID", orderID), zap.Error(err))
		return utils.ErrorResponse(
			ctx,
			apperrors.NewHttpError(
				http.StatusInternalServerError,
				"Не удалось получить историю заявки",
				err,
				nil,
			),
			c.logger,
		)
	}

	return utils.SuccessResponse(ctx, timeline, "История заявки успешно получена", http.StatusOK)
}
