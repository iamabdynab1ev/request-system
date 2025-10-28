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

// OrderHistoryController управляет запросами к истории заявок
type OrderHistoryController struct {
	historyService services.OrderHistoryServiceInterface
	orderService   services.OrderServiceInterface
	logger         *zap.Logger
}

// NewOrderHistoryController создает новый экземпляр OrderHistoryController
func NewOrderHistoryController(
	historyService services.OrderHistoryServiceInterface,
	orderService services.OrderServiceInterface,
	logger *zap.Logger,
) *OrderHistoryController {
	return &OrderHistoryController{
		historyService: historyService,
		orderService:   orderService,
		logger:         logger,
	}
}

// GetHistoryForOrder возвращает историю событий для указанной заявки
func (c *OrderHistoryController) GetHistoryForOrder(ctx echo.Context) error {
	reqCtx := ctx.Request().Context()

	// Парсим orderID
	orderID, err := strconv.ParseUint(ctx.Param("orderID"), 10, 64)
	if err != nil {
		c.logger.Error("Неверный формат ID заявки", zap.String("orderID", ctx.Param("orderID")), zap.Error(err))
		return utils.ErrorResponse(ctx, apperrors.NewHttpError(http.StatusBadRequest, "Неверный формат ID заявки", err, nil), c.logger)
	}

	// Парсим параметры пагинации
	limitStr := ctx.QueryParam("limit")
	offsetStr := ctx.QueryParam("offset")
	c.logger.Debug("Параметры пагинации", zap.String("limit", limitStr), zap.String("offset", offsetStr))

	// Проверяем доступ к заявке
	_, err = c.orderService.FindOrderByID(reqCtx, orderID)
	if err != nil {
		c.logger.Error("Ошибка проверки доступа к заявке", zap.Uint64("orderID", orderID), zap.Error(err))
		return utils.ErrorResponse(ctx, err, c.logger)
	}

	// Получаем историю
	timeline, err := c.historyService.GetTimelineByOrderID(reqCtx, orderID, limitStr, offsetStr)
	if err != nil {
		c.logger.Error("Не удалось получить историю заявки", zap.Uint64("orderID", orderID), zap.Error(err))
		return utils.ErrorResponse(ctx, apperrors.NewHttpError(http.StatusInternalServerError, "Не удалось получить историю заявки", err, nil), c.logger)
	}

	c.logger.Info("История заявки успешно получена", zap.Uint64("orderID", orderID), zap.Int("events", len(timeline)))
	return utils.SuccessResponse(ctx, timeline, "История заявки успешно получена", http.StatusOK)
}
