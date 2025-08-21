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
		return utils.ErrorResponse(ctx, apperrors.ErrBadRequest)
	}

	timeline, err := c.historyService.GetTimelineByOrderID(reqCtx, orderID)
	if err != nil {
		c.logger.Error("Ошибка при получении истории заявки", zap.Error(err), zap.Uint64("orderID", orderID))
		return utils.ErrorResponse(ctx, err)
	}

	return utils.SuccessResponse(ctx, timeline, "История заявки успешно получена", http.StatusOK)
}
