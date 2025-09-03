package controllers

import (
	"encoding/json"
	"net/http"
	"strconv"

	"request-system/internal/dto"
	"request-system/internal/services"
	apperrors "request-system/pkg/errors"
	"request-system/pkg/utils"

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

	orderListResponse, err := c.orderService.GetOrders(reqCtx, filter)
	if err != nil {
		c.logger.Error("GetOrders: ошибка при получении списка заявок", zap.Error(err))
		return utils.ErrorResponse(
			ctx,
			apperrors.NewHttpError(
				http.StatusInternalServerError,
				"Не удалось получить список заявок",
				err,
				nil,
			),
			c.logger,
		)
	}

	return utils.SuccessResponse(
		ctx,
		orderListResponse.List,
		"Список заявок успешно получен",
		http.StatusOK,
		orderListResponse.TotalCount,
	)
}

func (c *OrderController) FindOrder(ctx echo.Context) error {
	reqCtx := ctx.Request().Context()
	orderID, err := strconv.ParseUint(ctx.Param("id"), 10, 64)
	if err != nil {
		c.logger.Error("FindOrder: неверный формат ID", zap.String("id", ctx.Param("id")), zap.Error(err))
		return utils.ErrorResponse(ctx,
			apperrors.NewHttpError(
				http.StatusBadRequest,
				"Неверный формат ID заявки",
				err,
				map[string]interface{}{"param": ctx.Param("id")},
			),
			c.logger,
		)
	}

	order, err := c.orderService.FindOrderByID(reqCtx, orderID)
	if err != nil {
		c.logger.Warn("FindOrder: ошибка при поиске заявки по ID", zap.Uint64("orderID", orderID), zap.Error(err))
		return utils.ErrorResponse(ctx,
			apperrors.NewHttpError(
				http.StatusInternalServerError,
				"Не удалось найти заявку",
				err,
				nil,
			),
			c.logger,
		)
	}

	return utils.SuccessResponse(ctx, order, "Заявка успешно найдена", http.StatusOK)
}

func (c *OrderController) CreateOrder(ctx echo.Context) error {
	reqCtx := ctx.Request().Context()
	dataString := ctx.FormValue("data")

	if dataString == "" {
		c.logger.Warn("CreateOrder: поле 'data' отсутствует в form-data")
		return utils.ErrorResponse(
			ctx,
			apperrors.NewHttpError(
				http.StatusBadRequest,
				"Поле 'data' с JSON данными обязательно",
				nil,
				nil,
			),
			c.logger,
		)
	}

	file, err := ctx.FormFile("file")
	if err != nil && err != http.ErrMissingFile {
		c.logger.Error("CreateOrder: ошибка при чтении файла", zap.Error(err))
		return utils.ErrorResponse(
			ctx,
			apperrors.NewHttpError(
				http.StatusBadRequest,
				"Ошибка при чтении файла",
				err,
				nil,
			),
			c.logger,
		)
	}

	res, err := c.orderService.CreateOrder(reqCtx, dataString, file)
	if err != nil {
		c.logger.Error("CreateOrder: ошибка при создании заявки", zap.Error(err))
		return utils.ErrorResponse(
			ctx,
			apperrors.NewHttpError(
				http.StatusInternalServerError,
				"Не удалось создать заявку",
				err,
				nil,
			),
			c.logger,
		)
	}

	return utils.SuccessResponse(
		ctx,
		res,
		"Заявка успешно создана",
		http.StatusCreated,
	)
}

func (c *OrderController) UpdateOrder(ctx echo.Context) error {
	orderID, err := strconv.ParseUint(ctx.Param("id"), 10, 64)
	if err != nil {
		c.logger.Error("UpdateOrder: неверный формат ID", zap.String("id", ctx.Param("id")), zap.Error(err))
		return utils.ErrorResponse(ctx,
			apperrors.NewHttpError(
				http.StatusBadRequest,
				"Неверный формат ID заявки",
				err,
				map[string]interface{}{"param": ctx.Param("id")},
			),
			c.logger,
		)
	}

	dataString := ctx.FormValue("data")
	var dto dto.UpdateOrderDTO

	if dataString != "" {
		if err := json.Unmarshal([]byte(dataString), &dto); err != nil {
			c.logger.Error("UpdateOrder: некорректный JSON в поле 'data'", zap.Error(err))
			return utils.ErrorResponse(ctx,
				apperrors.NewHttpError(
					http.StatusBadRequest,
					"некорректный JSON в поле 'data'",
					err,
					nil,
				),
				c.logger,
			)
		}
	} else {
		// Позволяем обновлять и через чистое JSON-тело, если файл не прикреплен
		if err := ctx.Bind(&dto); err != nil {
			c.logger.Error("UpdateOrder: неверный формат запроса", zap.Error(err))
			return utils.ErrorResponse(ctx,
				apperrors.NewHttpError(
					http.StatusBadRequest,
					"неверный формат запроса",
					err,
					nil,
				),
				c.logger,
			)
		}
	}

	if err := ctx.Validate(&dto); err != nil {
		c.logger.Error("UpdateOrder: ошибка валидации данных", zap.Error(err))
		return utils.ErrorResponse(ctx, err, c.logger)
	}

	file, err := ctx.FormFile("file")
	if err != nil && err != http.ErrMissingFile {
		c.logger.Error("UpdateOrder: ошибка при чтении файла", zap.Error(err))
		return utils.ErrorResponse(ctx,
			apperrors.NewHttpError(
				http.StatusBadRequest,
				"Ошибка чтения файла",
				err,
				nil,
			),
			c.logger,
		)
	}

	updatedOrder, err := c.orderService.UpdateOrder(ctx.Request().Context(), orderID, dto, file)
	if err != nil {
		c.logger.Error("Ошибка при обновлении заявки", zap.Uint64("orderID", orderID), zap.Error(err))
		return utils.ErrorResponse(ctx,
			apperrors.NewHttpError(
				http.StatusInternalServerError,
				"Не удалось обновить заявку",
				err,
				nil,
			),
			c.logger,
		)
	}

	return utils.SuccessResponse(ctx, updatedOrder, "Заявка успешно обновлена", http.StatusOK)
}

func (c *OrderController) DeleteOrder(ctx echo.Context) error {
	orderID, err := strconv.ParseUint(ctx.Param("id"), 10, 64)
	if err != nil {
		c.logger.Error("DeleteOrder: неверный формат ID", zap.String("id", ctx.Param("id")), zap.Error(err))
		return utils.ErrorResponse(ctx,
			apperrors.NewHttpError(
				http.StatusBadRequest,
				"Неверный формат ID заявки",
				err,
				map[string]interface{}{"param": ctx.Param("id")},
			),
			c.logger,
		)
	}

	if err := c.orderService.DeleteOrder(ctx.Request().Context(), orderID); err != nil {
		c.logger.Error("DeleteOrder: ошибка при удалении заявки", zap.Uint64("orderID", orderID), zap.Error(err))
		return utils.ErrorResponse(ctx,
			apperrors.NewHttpError(
				http.StatusInternalServerError,
				"Не удалось удалить заявку",
				err,
				nil,
			),
			c.logger,
		)
	}

	return utils.SuccessResponse(ctx, struct{}{}, "Заявка успешно удалена", http.StatusOK)
}
