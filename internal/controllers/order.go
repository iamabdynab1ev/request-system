package controllers

import (
	"encoding/json"
	"mime/multipart"
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
	c.logger.Debug("Разобран фильтр из запроса", zap.Any("filter_struct", filter))

	orderListResponse, err := c.orderService.GetOrders(reqCtx, filter)
	if err != nil {
		c.logger.Error("Ошибка при получении списка заявок", zap.Error(err), zap.Any("filter", filter))
		return utils.ErrorResponse(ctx, apperrors.NewHttpError(
			http.StatusInternalServerError, "Не удалось получить список заявок", err, nil,
		), c.logger)
	}

	c.logger.Info("Список заявок успешно получен", zap.Int("количество", len(orderListResponse.List)), zap.Uint64("total_count", orderListResponse.TotalCount))

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
		return utils.ErrorResponse(ctx, apperrors.NewHttpError(
			http.StatusBadRequest, "Неверный формат ID заявки", err, map[string]interface{}{"param": ctx.Param("id")}),
			c.logger,
		)
	}

	order, err := c.orderService.FindOrderByID(reqCtx, orderID)
	if err != nil {
		c.logger.Warn("FindOrder: ошибка при поиске заявки по ID", zap.Uint64("orderID", orderID), zap.Error(err))
		return utils.ErrorResponse(ctx, err, c.logger)
	}

	c.logger.Info("Заявка найдена", zap.Uint64("orderID", orderID))
	return utils.SuccessResponse(ctx, order, "Заявка успешно найдена", http.StatusOK)
}

func (c *OrderController) CreateOrder(ctx echo.Context) error {
	reqCtx := ctx.Request().Context()

	dataString := ctx.FormValue("data")
	if dataString == "" {
		c.logger.Warn("CreateOrder: поле 'data' в form-data обязательно")
		return utils.ErrorResponse(ctx, apperrors.NewHttpError(
			http.StatusBadRequest, "Поле 'data' с JSON данными обязательно", nil, nil),
			c.logger,
		)
	}

	var createDTO dto.CreateOrderDTO
	if err := json.Unmarshal([]byte(dataString), &createDTO); err != nil {
		c.logger.Error("CreateOrder: некорректный JSON в поле 'data'", zap.Error(err), zap.String("data", dataString))
		return utils.ErrorResponse(ctx, apperrors.NewHttpError(
			http.StatusBadRequest, "Некорректный JSON в поле 'data'", err, nil),
			c.logger,
		)
	}

	if err := ctx.Validate(&createDTO); err != nil {
		c.logger.Error("CreateOrder: ошибка валидации данных", zap.Error(err), zap.Any("dto", createDTO))
		return utils.ErrorResponse(ctx, err, c.logger)
	}

	file, err := ctx.FormFile("file")
	if err != nil && err != http.ErrMissingFile {
		c.logger.Error("CreateOrder: ошибка при чтении файла", zap.Error(err))
		return utils.ErrorResponse(ctx, apperrors.NewHttpError(
			http.StatusBadRequest, "Ошибка при чтении файла", err, nil),
			c.logger,
		)
	}

	res, err := c.orderService.CreateOrder(reqCtx, createDTO, file)
	if err != nil {
		c.logger.Error("CreateOrder: сервис вернул ошибку", zap.Error(err), zap.Any("dto", createDTO))
		return utils.ErrorResponse(ctx, err, c.logger)
	}

	c.logger.Info("Заявка создана", zap.Uint64("orderID", res.ID))
	return utils.SuccessResponse(
		ctx,
		res,
		"Заявка успешно создана",
		http.StatusCreated,
	)
}

func (c *OrderController) UpdateOrder(ctx echo.Context) error {
	reqCtx := ctx.Request().Context()
	orderID, err := strconv.ParseUint(ctx.Param("id"), 10, 64)
	if err != nil {
		c.logger.Error("UpdateOrder: неверный формат ID", zap.String("id", ctx.Param("id")), zap.Error(err))
		return utils.ErrorResponse(ctx, apperrors.NewHttpError(
			http.StatusBadRequest, "Неверный формат ID заявки", err, map[string]interface{}{"param": ctx.Param("id")}),
			c.logger,
		)
	}

	dataString := ctx.FormValue("data")
	var updateDTO dto.UpdateOrderDTO
	rawRequestBody := []byte(dataString)

	if dataString != "" {
		if err := json.Unmarshal(rawRequestBody, &updateDTO); err != nil {
			c.logger.Error("UpdateOrder: некорректный JSON в поле 'data'", zap.Error(err), zap.String("data", dataString))
			return utils.ErrorResponse(ctx, apperrors.NewHttpError(http.StatusBadRequest, "Некорректный JSON в поле 'data'", err, nil), c.logger)
		}
	} else {
		rawRequestBody = []byte("{}")
	}

	if err := ctx.Validate(&updateDTO); err != nil {
		c.logger.Error("UpdateOrder: ошибка валидации DTO", zap.Error(err), zap.Any("dto", updateDTO))
		return utils.ErrorResponse(ctx, err, c.logger)
	}
	var fileHeader *multipart.FileHeader
	file, err := ctx.FormFile("file")
	if err != nil && err != http.ErrMissingFile {

		c.logger.Error("UpdateOrder: ошибка при чтении поля 'file'", zap.Error(err))
		return utils.ErrorResponse(ctx, apperrors.NewHttpError(http.StatusBadRequest, "Ошибка чтения файла", err, nil), c.logger)
	} else if err == http.ErrMissingFile {

		file, err = ctx.FormFile("comment_attachment")
		if err != nil && err != http.ErrMissingFile {
			c.logger.Error("UpdateOrder: ошибка при чтении поля 'comment_attachment'", zap.Error(err))
			return utils.ErrorResponse(ctx, apperrors.NewHttpError(http.StatusBadRequest, "Ошибка чтения файла", err, nil), c.logger)
		}
	}

	if file != nil {
		fileHeader = file
	}

	updatedOrder, err := c.orderService.UpdateOrder(reqCtx, orderID, updateDTO, fileHeader, rawRequestBody)
	if err != nil {
		c.logger.Error("Ошибка при обновлении заявки", zap.Uint64("orderID", orderID), zap.Error(err), zap.Any("dto", updateDTO))
		return utils.ErrorResponse(ctx, err, c.logger)
	}

	c.logger.Info("Заявка обновлена", zap.Uint64("orderID", orderID))
	return utils.SuccessResponse(ctx, updatedOrder, "Заявка успешно обновлена", http.StatusOK)
}

func (c *OrderController) DeleteOrder(ctx echo.Context) error {
	reqCtx := ctx.Request().Context()
	orderID, err := strconv.ParseUint(ctx.Param("id"), 10, 64)
	if err != nil {
		c.logger.Error("DeleteOrder: неверный формат ID", zap.String("id", ctx.Param("id")), zap.Error(err))
		return utils.ErrorResponse(ctx, apperrors.NewHttpError(
			http.StatusBadRequest, "Неверный формат ID заявки", err, map[string]interface{}{"param": ctx.Param("id")}),
			c.logger,
		)
	}

	if err := c.orderService.DeleteOrder(reqCtx, orderID); err != nil {
		c.logger.Error("DeleteOrder: ошибка при удалении заявки", zap.Uint64("orderID", orderID), zap.Error(err))
		return utils.ErrorResponse(ctx, apperrors.NewHttpError(
			http.StatusInternalServerError, "Не удалось удалить заявку", err, nil),
			c.logger,
		)
	}

	c.logger.Info("Заявка удалена", zap.Uint64("orderID", orderID))
	return utils.SuccessResponse(ctx, struct{}{}, "Заявка успешно удалена", http.StatusOK)
}
