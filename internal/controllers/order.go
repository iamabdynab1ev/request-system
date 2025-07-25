package controllers

import (
	"encoding/json"
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

func (c *OrderController) CreateOrder(ctx echo.Context) error {
	dataString := ctx.FormValue("data")
	if dataString == "" {
		return utils.ErrorResponse(ctx, echo.NewHTTPError(http.StatusBadRequest, "поле 'data' с JSON данными не найдено"))
	}

	var tempDTO dto.CreateOrderDTO
	if err := json.Unmarshal([]byte(dataString), &tempDTO); err != nil {
		c.logger.Error("ошибка разбора JSON из поля 'data'", zap.Error(err), zap.String("data", dataString))
		return utils.ErrorResponse(ctx, echo.NewHTTPError(http.StatusBadRequest, "некорректный JSON в поле 'data'"))
	}

	if err := ctx.Validate(&tempDTO); err != nil {
		c.logger.Error("ошибка валидации данных заявки", zap.Error(err))
		return utils.ErrorResponse(ctx, err)
	}
	c.logger.Info("[OrderController] Пытаюсь получить файл из формы (ключ 'file')...")

	file, err := ctx.FormFile("file")
	if err != nil {
		if err != http.ErrMissingFile {
			c.logger.Error("[OrderController] КРИТИЧЕСКАЯ ОШИБКА при чтении файла из формы", zap.Error(err))
			return utils.ErrorResponse(ctx, err)
		}
		c.logger.Warn("[OrderController] Файл в запросе НЕ найден. Передаю nil в сервис.", zap.Error(err))
	} else {
		c.logger.Info("[OrderController] УСПЕХ! Файл получен из формы", zap.String("filename", file.Filename), zap.Int64("size", file.Size))
	}

	res, err := c.orderService.CreateOrder(ctx.Request().Context(), dataString, file)
	if err != nil {
		c.logger.Error("ПОЛУЧЕНА ОШИБКА ИЗ СЕРВИСА", zap.Error(err))

		return ctx.JSON(http.StatusInternalServerError, map[string]interface{}{
			"status":  false,
			"message": "Произошла ошибка. Исходный текст ошибки: " + err.Error(),
		})
	}

	return utils.SuccessResponse(ctx, res, "Заявка успешно создана", http.StatusCreated)
}

func (c *OrderController) DelegateOrder(ctx echo.Context) error {
	id, err := strconv.ParseUint(ctx.Param("id"), 10, 64)
	if err != nil {
		return utils.ErrorResponse(ctx, echo.NewHTTPError(http.StatusBadRequest, "Invalid ID"))
	}

	dataString := ctx.FormValue("data")
	if dataString == "" {
		return utils.ErrorResponse(ctx, echo.NewHTTPError(http.StatusBadRequest, "поле 'data' с JSON данными не найдено"))
	}

	var dto dto.DelegateOrderDTO
	if err = json.Unmarshal([]byte(dataString), &dto); err != nil {
		return utils.ErrorResponse(ctx, echo.NewHTTPError(http.StatusBadRequest, "некорректный JSON в поле 'data'"))
	}
	if err = ctx.Validate(&dto); err != nil {
		return utils.ErrorResponse(ctx, err)
	}

	file, err := ctx.FormFile("file")
	if err != nil && err != http.ErrMissingFile {
		return utils.ErrorResponse(ctx, err)
	}

	res, err := c.orderService.DelegateOrder(ctx.Request().Context(), id, dto, file)
	if err != nil {
		return utils.ErrorResponse(ctx, err)
	}

	return utils.SuccessResponse(ctx, res, "Order updated successfully", http.StatusOK)
}
func (c *OrderController) GetOrders(ctx echo.Context) error {
	return utils.ErrorResponse(ctx, echo.NewHTTPError(http.StatusNotImplemented, "метод GetOrders еще не реализован"))
}

func (c *OrderController) FindOrder(ctx echo.Context) error {
	return utils.ErrorResponse(ctx, echo.NewHTTPError(http.StatusNotImplemented, "метод FindOrder еще не реализован"))
}

func (c *OrderController) UpdateOrder(ctx echo.Context) error {
	return utils.ErrorResponse(ctx, echo.NewHTTPError(http.StatusNotImplemented, "метод UpdateOrder еще не реализован"))
}

func (c *OrderController) DeleteOrder(ctx echo.Context) error {
	return utils.ErrorResponse(ctx, echo.NewHTTPError(http.StatusNotImplemented, "метод DeleteOrder еще не реализован"))
}
