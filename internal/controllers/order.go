package controllers

import (
	"net/http"
	"request-system/internal/dto"
	"request-system/internal/services"
	"request-system/pkg/utils"
	"strconv"

	"github.com/labstack/echo/v4"
	"go.uber.org/zap"
)

type OrderController struct {
	orderService services.OrderServiceInterface
	logger       *zap.Logger
}

func NewOrderController(
	orderService services.OrderServiceInterface,
	logger *zap.Logger,
) *OrderController {
	return &OrderController{
		orderService: orderService,
		logger:       logger,
	}
}

func (c *OrderController) GetOrders(ctx echo.Context) error {
	reqCtx := ctx.Request().Context()
	limit, offset, _ := utils.ParsePaginationParams(ctx.QueryParams())

	res, total, err := c.orderService.GetOrders(reqCtx, limit, offset)
	if err != nil {
		c.logger.Error("ошибка при получении списка заявок", zap.Error(err))
		return utils.ErrorResponse(ctx, err)
	}
	return utils.SuccessResponse(ctx, res, "Список заявок успешно получен", http.StatusOK, total)
}

func (c *OrderController) FindOrder(ctx echo.Context) error {
	reqCtx := ctx.Request().Context()
	id, err := strconv.ParseUint(ctx.Param("id"), 10, 64)
	if err != nil {
		c.logger.Error("Некорректный ID заявки", zap.Error(err))
		return utils.ErrorResponse(ctx, echo.NewHTTPError(http.StatusBadRequest, "Некорректный ID заявки"))
	}

	res, err := c.orderService.FindOrder(reqCtx, id)
	if err != nil {
		c.logger.Error("Ошибка при поиске заявки", zap.Error(err), zap.Uint64("id", id))
		return utils.ErrorResponse(ctx, err)
	}
	return utils.SuccessResponse(ctx, res, "Заявка успешно найдена", http.StatusOK)
}

func (c *OrderController) CreateOrder(ctx echo.Context) error {
	reqCtx := ctx.Request().Context()
	var dto dto.CreateOrderDTO
	if err := ctx.Bind(&dto); err != nil {
		c.logger.Error("Неверный запрос", zap.Error(err))
		return utils.ErrorResponse(ctx, err)
	}
	if err := ctx.Validate(&dto); err != nil {
		c.logger.Error("Ошибка валидации данных заявки", zap.Error(err))
		return utils.ErrorResponse(ctx, err)
	}
	res, err := c.orderService.CreateOrder(reqCtx, dto)
	if err != nil {
		c.logger.Error("Ошибка при создании заявки", zap.Error(err))
		return utils.ErrorResponse(ctx, err)
	}
	return utils.SuccessResponse(ctx, res, "Заявка успешно создана", http.StatusCreated)
}

func (c *OrderController) UpdateOrder(ctx echo.Context) error {
	reqCtx := ctx.Request().Context()
	id, err := strconv.ParseUint(ctx.Param("id"), 10, 64)
	if err != nil {
		return utils.ErrorResponse(ctx, echo.NewHTTPError(http.StatusBadRequest, "Некорректный ID"))
	}
	var dto dto.UpdateOrderDTO
	if err := ctx.Bind(&dto); err != nil {
		return utils.ErrorResponse(ctx, err)
	}
	dto.ID = int(id)

	err = c.orderService.UpdateOrder(reqCtx, id, dto)
	if err != nil {
		return utils.ErrorResponse(ctx, err)
	}
	return utils.SuccessResponse(ctx, nil, "Заявка успешно обновлена", http.StatusOK)
}

func (c *OrderController) SoftDeleteOrder(ctx echo.Context) error {
	reqCtx := ctx.Request().Context()
	id, err := strconv.ParseUint(ctx.Param("id"), 10, 64)
	if err != nil {
		return utils.ErrorResponse(ctx, echo.NewHTTPError(http.StatusBadRequest, "Некорректный ID"))
	}
	err = c.orderService.SoftDeleteOrder(reqCtx, id)
	if err != nil {
		return utils.ErrorResponse(ctx, err)
	}
	return utils.SuccessResponse(ctx, struct{}{}, "Заявка успешно удалена", http.StatusOK)
}
