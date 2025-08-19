package controllers

import (
	"net/http"
	"request-system/internal/dto"
	"request-system/internal/services"
	apperrors "request-system/pkg/errors"
	"request-system/pkg/utils"
	"strconv"

	"github.com/labstack/echo/v4"
	"go.uber.org/zap"
)

type PriorityController struct {
	priorityService services.PriorityServiceInterface
	logger          *zap.Logger
}

func NewPriorityController(priorityService services.PriorityServiceInterface, logger *zap.Logger) *PriorityController {
	return &PriorityController{priorityService: priorityService, logger: logger}
}

func (c *PriorityController) GetPriorities(ctx echo.Context) error {
	reqCtx := ctx.Request().Context()
	filter := utils.ParseFilterFromQuery(ctx.Request().URL.Query())

	res, err := c.priorityService.GetPriorities(reqCtx, uint64(filter.Limit), uint64(filter.Offset), filter.Search)
	if err != nil {
		c.logger.Error("Ошибка получения списка приоритетов", zap.Error(err))
		return utils.ErrorResponse(ctx, err)
	}

	return utils.SuccessResponse(ctx, res.List, "Приоритеты успешно получены", http.StatusOK, res.Pagination.TotalCount)
}

func (c *PriorityController) FindPriority(ctx echo.Context) error {
	reqCtx := ctx.Request().Context()
	id, err := strconv.ParseUint(ctx.Param("id"), 10, 64)
	if err != nil {
		return utils.ErrorResponse(ctx, apperrors.NewHttpError(http.StatusBadRequest, "Неверный ID", err))
	}

	res, err := c.priorityService.FindPriority(reqCtx, id)
	if err != nil {
		return utils.ErrorResponse(ctx, err)
	}
	return utils.SuccessResponse(ctx, res, "Приоритет успешно найден", http.StatusOK)
}

func (c *PriorityController) CreatePriority(ctx echo.Context) error {
	var dto dto.CreatePriorityDTO
	if err := ctx.Bind(&dto); err != nil {
		return utils.ErrorResponse(ctx, apperrors.NewHttpError(http.StatusBadRequest, "Неверный формат запроса", err))
	}
	if err := ctx.Validate(&dto); err != nil {
		return utils.ErrorResponse(ctx, err)
	}

	res, err := c.priorityService.CreatePriority(ctx.Request().Context(), dto)
	if err != nil {
		return utils.ErrorResponse(ctx, err)
	}
	return utils.SuccessResponse(ctx, res, "Приоритет успешно создан", http.StatusCreated)
}

func (c *PriorityController) UpdatePriority(ctx echo.Context) error {
	id, err := strconv.ParseUint(ctx.Param("id"), 10, 64)
	if err != nil {
		return utils.ErrorResponse(ctx, apperrors.NewHttpError(http.StatusBadRequest, "Неверный ID", err))
	}

	var dto dto.UpdatePriorityDTO
	if err := ctx.Bind(&dto); err != nil {
		return utils.ErrorResponse(ctx, apperrors.NewHttpError(http.StatusBadRequest, "Неверный формат запроса", err))
	}
	if err := ctx.Validate(&dto); err != nil {
		return utils.ErrorResponse(ctx, err)
	}

	res, err := c.priorityService.UpdatePriority(ctx.Request().Context(), id, dto)
	if err != nil {
		return utils.ErrorResponse(ctx, err)
	}
	return utils.SuccessResponse(ctx, res, "Приоритет успешно обновлен", http.StatusOK)
}

func (c *PriorityController) DeletePriority(ctx echo.Context) error {
	id, err := strconv.ParseUint(ctx.Param("id"), 10, 64)
	if err != nil {
		return utils.ErrorResponse(ctx, apperrors.NewHttpError(http.StatusBadRequest, "Неверный ID", err))
	}

	err = c.priorityService.DeletePriority(ctx.Request().Context(), id)
	if err != nil {
		return utils.ErrorResponse(ctx, err)
	}
	return utils.SuccessResponse(ctx, struct{}{}, "Приоритет успешно удален", http.StatusOK)
}
