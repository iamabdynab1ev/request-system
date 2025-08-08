package controllers

import (
	"fmt"
	"net/http"
	"strconv"

	"request-system/internal/dto"
	"request-system/internal/services"
	apperrors "request-system/pkg/errors"
	"request-system/pkg/utils"

	"github.com/labstack/echo/v4"
	"go.uber.org/zap"
)

type OrderDocumentController struct {
	orderDocumentService *services.OrderDocumentService
	logger               *zap.Logger
}

func NewOrderDocumentController(
	orderDocumentService *services.OrderDocumentService,
	logger *zap.Logger,
) *OrderDocumentController {
	return &OrderDocumentController{
		orderDocumentService: orderDocumentService,
		logger:               logger,
	}
}

func (c *OrderDocumentController) GetOrderDocuments(ctx echo.Context) error {
	reqCtx := ctx.Request().Context()

	res, err := c.orderDocumentService.GetOrderDocuments(reqCtx, 6, 10)
	if err != nil {
		c.logger.Error("Ошибка при получении списка документов заказа", zap.Error(err))
		return utils.ErrorResponse(
			ctx,
			err,
		)
	}

	return utils.SuccessResponse(
		ctx,
		res,
		"Successfully",
		http.StatusOK,
	)
}

func (c *OrderDocumentController) FindOrderDocument(ctx echo.Context) error {
	reqCtx := ctx.Request().Context()

	id, err := strconv.ParseUint(ctx.Param("id"), 10, 64)
	if err != nil {
		c.logger.Error("Ошибка парсинга ID документа заказа из URL", zap.Error(err))
		return utils.ErrorResponse(ctx, fmt.Errorf("invalid order document ID format: %w", apperrors.ErrBadRequest))
	}

	res, err := c.orderDocumentService.FindOrderDocument(reqCtx, id)
	if err != nil {
		c.logger.Error("Ошибка при поиске документа заказа по ID", zap.Uint64("id", id), zap.Error(err))
		return utils.ErrorResponse(
			ctx,
			err,
		)
	}

	return utils.SuccessResponse(
		ctx,
		res,
		"Successfully",
		http.StatusOK,
	)
}

func (c *OrderDocumentController) CreateOrderDocument(ctx echo.Context) error {
	reqCtx := ctx.Request().Context()

	var dto dto.CreateOrderDocumentDTO
	if err := ctx.Bind(&dto); err != nil {
		c.logger.Error("Ошибка при связывании запроса для создания документа заказа", zap.Error(err))
		return utils.ErrorResponse(ctx, apperrors.ErrBadRequest)
	}

	if err := ctx.Validate(&dto); err != nil {
		c.logger.Error("Ошибка при валидации данных для создания документа заказа", zap.Error(err))
		return utils.ErrorResponse(ctx, apperrors.ErrBadRequest)
	}

	res, err := c.orderDocumentService.CreateOrderDocument(reqCtx, dto)
	if err != nil {
		c.logger.Error("Ошибка при создании документа заказа в сервисе", zap.Error(err))
		return utils.ErrorResponse(
			ctx,
			err,
		)
	}

	return utils.SuccessResponse(
		ctx,
		res,
		"Successfully",
		http.StatusOK,
	)
}

func (c *OrderDocumentController) UpdateOrderDocument(ctx echo.Context) error {
	reqCtx := ctx.Request().Context()

	idFromURL, err := strconv.ParseUint(ctx.Param("id"), 10, 64)
	if err != nil {
		c.logger.Error("Ошибка парсинга ID документа заказа из URL для обновления", zap.Error(err))
		return utils.ErrorResponse(ctx, fmt.Errorf("invalid order document ID format in URL: %w", apperrors.ErrBadRequest))
	}

	var dto dto.UpdateOrderDocumentDTO
	if err := ctx.Bind(&dto); err != nil {
		c.logger.Error("Ошибка при связывании запроса для обновления документа заказа", zap.Error(err))
		return utils.ErrorResponse(ctx, fmt.Errorf("request binding failed: %w", apperrors.ErrBadRequest))
	}

	if err := ctx.Validate(&dto); err != nil {
		c.logger.Error("Ошибка при валидации данных для обновления документа заказа", zap.Error(err))
		return utils.ErrorResponse(ctx, fmt.Errorf("validation failed: %w", apperrors.ErrBadRequest))
	}

	dto.ID = int(idFromURL)

	res, err := c.orderDocumentService.UpdateOrderDocument(reqCtx, uint64(dto.ID), dto)
	if err != nil {
		c.logger.Error("Ошибка при обновлении документа заказа в сервисе", zap.Uint64("id", idFromURL), zap.Error(err))
		return utils.ErrorResponse(
			ctx,
			err,
		)
	}

	return utils.SuccessResponse(
		ctx,
		res,
		"Successfully",
		http.StatusOK,
	)
}

func (c *OrderDocumentController) DeleteOrderDocument(ctx echo.Context) error {
	reqCtx := ctx.Request().Context()

	id, err := strconv.ParseUint(ctx.Param("id"), 10, 64)
	if err != nil {
		c.logger.Error("Ошибка парсинга ID документа заказа из URL для удаления", zap.Error(err))
		return utils.ErrorResponse(ctx, fmt.Errorf("invalid order document ID format: %w", apperrors.ErrBadRequest))
	}

	err = c.orderDocumentService.DeleteOrderDocument(reqCtx, id)
	if err != nil {
		c.logger.Error("Ошибка при удалении документа заказа в сервисе", zap.Uint64("id", id), zap.Error(err))
		return utils.ErrorResponse(
			ctx,
			err,
		)
	}

	return utils.SuccessResponse(
		ctx,
		struct{}{},
		"Successfully",
		http.StatusOK,
	)
}
