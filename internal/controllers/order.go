package controllers

import (
	"encoding/json"
	"net/http"
	"request-system/internal/dto"
	"request-system/internal/services"
	apperrors "request-system/pkg/errors"
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
	filter := utils.ParseFilterFromQuery(ctx.Request().URL.Query())

	// Сервис сам извлечет ID пользователя и его права из контекста
	orderListResponse, err := c.orderService.GetOrders(reqCtx, filter)
	if err != nil {
		c.logger.Error("Ошибка при получении списка заявок", zap.Error(err))
		return utils.ErrorResponse(ctx, err)
	}

	return utils.SuccessResponse(ctx, orderListResponse.List, "Заявки успешно получены", http.StatusOK, orderListResponse.TotalCount)
}

func (c *OrderController) FindOrder(ctx echo.Context) error {
	reqCtx := ctx.Request().Context()
	orderID, err := strconv.ParseUint(ctx.Param("id"), 10, 64)
	if err != nil {
		return utils.ErrorResponse(ctx, apperrors.NewHttpError(http.StatusBadRequest, "Неверный формат ID", err))
	}

	order, err := c.orderService.FindOrderByID(reqCtx, orderID)
	if err != nil {
		c.logger.Warn("Ошибка при поиске заявки по ID", zap.Uint64("orderID", orderID), zap.Error(err))
		return utils.ErrorResponse(ctx, err)
	}

	return utils.SuccessResponse(ctx, order, "Заявка успешно найдена", http.StatusOK)
}

func (c *OrderController) CreateOrder(ctx echo.Context) error {
	dataString := ctx.FormValue("data")
	if dataString == "" {
		return utils.ErrorResponse(ctx, apperrors.NewHttpError(http.StatusBadRequest, "поле 'data' с JSON данными обязательно", nil))
	}

	file, err := ctx.FormFile("file")
	if err != nil && err != http.ErrMissingFile {
		return utils.ErrorResponse(ctx, apperrors.NewHttpError(http.StatusBadRequest, "Ошибка чтения файла", err))
	}

	res, err := c.orderService.CreateOrder(ctx.Request().Context(), dataString, file)
	if err != nil {
		c.logger.Error("Ошибка при создании заявки", zap.Error(err))
		return utils.ErrorResponse(ctx, err)
	}
	return utils.SuccessResponse(ctx, res, "Заявка успешно создана", http.StatusCreated)
}

func (c *OrderController) UpdateOrder(ctx echo.Context) error {
	orderID, err := strconv.ParseUint(ctx.Param("id"), 10, 64)
	if err != nil {
		return utils.ErrorResponse(ctx, apperrors.NewHttpError(http.StatusBadRequest, "Неверный формат ID", err))
	}

	dataString := ctx.FormValue("data")
	var dto dto.UpdateOrderDTO

	if dataString != "" {
		if err := json.Unmarshal([]byte(dataString), &dto); err != nil {
			return utils.ErrorResponse(ctx, apperrors.NewHttpError(http.StatusBadRequest, "некорректный JSON в поле 'data'", err))
		}
	} else {
		// Позволяем обновлять и через чистое JSON-тело, если файл не прикреплен
		if err := ctx.Bind(&dto); err != nil {
			return utils.ErrorResponse(ctx, apperrors.NewHttpError(http.StatusBadRequest, "неверный формат запроса", err))
		}
	}

	if err := ctx.Validate(&dto); err != nil {
		return utils.ErrorResponse(ctx, err)
	}

	file, err := ctx.FormFile("file")
	if err != nil && err != http.ErrMissingFile {
		return utils.ErrorResponse(ctx, apperrors.NewHttpError(http.StatusBadRequest, "Ошибка чтения файла", err))
	}

	updatedOrder, err := c.orderService.UpdateOrder(ctx.Request().Context(), orderID, dto, file)
	if err != nil {
		c.logger.Error("Ошибка при обновлении заявки", zap.Uint64("orderID", orderID), zap.Error(err))
		return utils.ErrorResponse(ctx, err)
	}

	return utils.SuccessResponse(ctx, updatedOrder, "Заявка успешно обновлена", http.StatusOK)
}

func (c *OrderController) DeleteOrder(ctx echo.Context) error {
	orderID, err := strconv.ParseUint(ctx.Param("id"), 10, 64)
	if err != nil {
		return utils.ErrorResponse(ctx, apperrors.NewHttpError(http.StatusBadRequest, "Неверный формат ID", err))
	}

	if err := c.orderService.DeleteOrder(ctx.Request().Context(), orderID); err != nil {
		c.logger.Error("Ошибка при удалении заявки", zap.Uint64("orderID", orderID), zap.Error(err))
		return utils.ErrorResponse(ctx, err)
	}

	return utils.SuccessResponse(ctx, nil, "Заявка успешно удалена", http.StatusOK)
}
