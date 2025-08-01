package controllers

import (
	"errors"
	"net/http"
	"request-system/internal/dto"
	"request-system/internal/services"
	apperrors "request-system/pkg/errors"
	"request-system/pkg/utils"
	"strconv"

	"github.com/labstack/echo/v4"
	"go.uber.org/zap"
)

type StatusController struct {
	statusService *services.StatusService
	logger        *zap.Logger
}

func NewStatusController(
	statusService *services.StatusService,
	logger *zap.Logger,
) *StatusController {
	return &StatusController{
		statusService: statusService,
		logger:        logger,
	}
}

func (c *StatusController) GetStatuses(ctx echo.Context) error {
	reqCtx := ctx.Request().Context()
	filter := utils.ParseFilterFromQuery(ctx.Request().URL.Query())

	statuses, total, err := c.statusService.GetStatuses(reqCtx, uint64(filter.Limit), uint64(filter.Offset))
	if err != nil {
		c.logger.Error("ошибка при получении статусов", zap.Error(err))
		return utils.ErrorResponse(ctx, err)
	}

	if statuses == nil {
		statuses = make([]dto.StatusDTO, 0)
	}

	return utils.SuccessResponse(ctx, statuses, "Successfully", http.StatusOK, total)
}

func (c *StatusController) FindStatus(ctx echo.Context) error {
	reqCtx := ctx.Request().Context()
	id, err := strconv.ParseUint(ctx.Param("id"), 10, 64)
	if err != nil {
		return utils.ErrorResponse(ctx, apperrors.NewHttpError(http.StatusBadRequest, "Некорректный ID", err))
	}

	res, err := c.statusService.FindStatus(reqCtx, id)
	if err != nil {
		return utils.ErrorResponse(ctx, err)
	}

	return utils.SuccessResponse(ctx, res, "Successfully", http.StatusOK)
}

func (c *StatusController) FindByCode(ctx echo.Context) error {
	reqCtx := ctx.Request().Context()
	code := ctx.Param("code")

	res, err := c.statusService.FindByCode(reqCtx, code)
	if err != nil {
		return utils.ErrorResponse(ctx, err)
	}

	return utils.SuccessResponse(ctx, res, "Successfully", http.StatusOK)
}

func (c *StatusController) CreateStatus(ctx echo.Context) error {
	reqCtx := ctx.Request().Context()
	var dto dto.CreateStatusDTO
	if err := ctx.Bind(&dto); err != nil {
		return utils.ErrorResponse(ctx, err)
	}
	if err := ctx.Validate(&dto); err != nil {
		return utils.ErrorResponse(ctx, err)
	}

	createdStatus, err := c.statusService.CreateStatus(reqCtx, dto)
	if err != nil {
		return utils.ErrorResponse(ctx, err)
	}

	return utils.SuccessResponse(ctx, createdStatus, "Статус успешно создан", http.StatusCreated)
}

func (c *StatusController) UpdateStatus(ctx echo.Context) error {
	reqCtx := ctx.Request().Context()
	id, err := strconv.ParseUint(ctx.Param("id"), 10, 64)
	if err != nil {
		return utils.ErrorResponse(ctx, apperrors.NewHttpError(http.StatusBadRequest, "Invalid ID format", err))
	}

	var dto dto.UpdateStatusDTO
	if err := ctx.Bind(&dto); err != nil {
		return utils.ErrorResponse(ctx, err)
	}
	if err := ctx.Validate(&dto); err != nil {
		return utils.ErrorResponse(ctx, err)
	}

	updatedStatus, err := c.statusService.UpdateStatus(reqCtx, id, dto)
	if err != nil {
		return utils.ErrorResponse(ctx, err)
	}

	return utils.SuccessResponse(ctx, updatedStatus, "Статус успешно обновлен", http.StatusOK)
}

func (c *StatusController) DeleteStatus(ctx echo.Context) error {
	reqCtx := ctx.Request().Context()
	id, err := strconv.ParseUint(ctx.Param("id"), 10, 64)
	if err != nil {
		return utils.ErrorResponse(ctx, apperrors.NewHttpError(http.StatusBadRequest, "Invalid ID format", err))
	}

	err = c.statusService.DeleteStatus(reqCtx, id)
	if err != nil {
		var httpErr *apperrors.HttpError
		if errors.As(err, &httpErr) {
			return utils.ErrorResponse(ctx, httpErr)
		}
		c.logger.Error("ошибка при удалении статуса", zap.Error(err), zap.Uint64("id", id))
		return utils.ErrorResponse(ctx, err)
	}

	return utils.SuccessResponse(ctx, nil, "Статус успешно удален", http.StatusOK)
}
