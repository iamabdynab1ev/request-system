package controllers

import (
	"net/http"
	"strconv"

	"request-system/internal/services"
	"request-system/pkg/utils"

	"github.com/labstack/echo/v4"
	"go.uber.org/zap"
)

type AttachmentController struct {
	attachmentService services.AttachmentServiceInterface
	logger            *zap.Logger
}

func NewAttachmentController(
	attachmentService services.AttachmentServiceInterface,
	logger *zap.Logger,
) *AttachmentController {
	return &AttachmentController{
		attachmentService: attachmentService,
		logger:            logger,
	}
}

func (c *AttachmentController) GetAttachmentsByOrder(ctx echo.Context) error {
	orderID, err := strconv.ParseUint(ctx.QueryParam("order_id"), 10, 64)
	if err != nil || orderID == 0 {
		return utils.ErrorResponse(ctx, echo.NewHTTPError(http.StatusBadRequest, "некорректный или отсутствующий 'order_id'"))
	}

	res, err := c.attachmentService.GetAttachmentsByOrderID(ctx.Request().Context(), orderID)
	if err != nil {
		c.logger.Error("ошибка при получении вложений", zap.Error(err), zap.Uint64("orderID", orderID))
		return utils.ErrorResponse(ctx, err)
	}

	return utils.SuccessResponse(ctx, res, "Successfully", http.StatusOK)
}

func (c *AttachmentController) DeleteAttachment(ctx echo.Context) error {
	id, err := strconv.ParseUint(ctx.Param("id"), 10, 64)
	if err != nil {
		c.logger.Error("неверный ID вложения", zap.Error(err))
		return utils.ErrorResponse(ctx, echo.NewHTTPError(http.StatusBadRequest, "неверный ID вложения"))
	}

	err = c.attachmentService.DeleteAttachment(ctx.Request().Context(), id)
	if err != nil {
		c.logger.Error("ошибка при удалении вложения", zap.Error(err))
		return utils.ErrorResponse(ctx, err)
	}

	return utils.SuccessResponse(ctx, nil, "Attachment successfully deleted", http.StatusOK)
}
