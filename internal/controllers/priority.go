package controllers

import (
	"fmt"
	"net/http"
	"strconv"

	"request-system/internal/dto"
	"request-system/internal/services"
	apperrors "request-system/pkg/errors"
	"request-system/pkg/utils"

	"go.uber.org/zap"

	"github.com/labstack/echo/v4"
)

type PriorityController struct {
	priorityService *services.PriorityService
	logger          *zap.Logger
}

func NewPriorityController(priorityService *services.PriorityService,
	logger *zap.Logger,
) *PriorityController {
	return &PriorityController{
		priorityService: priorityService,
		logger:          logger,
	}
}

func (c *PriorityController) GetPriorities(ctx echo.Context) error {
	reqCtx := ctx.Request().Context()

	res, err := c.priorityService.GetPriorities(reqCtx, 6, 10)
	if err != nil {
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

func (c *PriorityController) FindPriority(ctx echo.Context) error {
	reqCtx := ctx.Request().Context()

	id, err := strconv.ParseUint(ctx.Param("id"), 10, 64)
	if err != nil {
		return utils.ErrorResponse(ctx, fmt.Errorf("invalid prorety ID format: %w", apperrors.ErrBadRequest))
	}

	res, err := c.priorityService.FindPriority(reqCtx, id)
	if err != nil {
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

func (c *PriorityController) CreatePriority(ctx echo.Context) error {
	reqCtx := ctx.Request().Context()

	var dto dto.CreatePriorityDTO
	if err := ctx.Bind(&dto); err != nil {
		c.logger.Error("неверный запрос", zap.Error(err))
		return utils.ErrorResponse(ctx, fmt.Errorf("request binding failed: %w", apperrors.ErrBadRequest))
	}

	if err := ctx.Validate(&dto); err != nil {
		c.logger.Error("Ощибка при валидации данных: ", zap.Error(err))
		return utils.ErrorResponse(ctx, err)
	}

	res, err := c.priorityService.CreatePriority(reqCtx, dto)
	if err != nil {
		c.logger.Error("Ощибка при создание: ", zap.Error(err))
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

func (c *PriorityController) UpdatePriority(ctx echo.Context) error {
	reqCtx := ctx.Request().Context()

	id, err := strconv.ParseUint(ctx.Param("id"), 10, 64)
	if err != nil {
		return utils.ErrorResponse(ctx, fmt.Errorf("invalid prorety ID format: %w", apperrors.ErrBadRequest))
	}

	var dto dto.UpdatePriorityDTO
	if err := ctx.Bind(&dto); err != nil {
		return utils.ErrorResponse(ctx, fmt.Errorf("request binding failed: %w", apperrors.ErrBadRequest))
	}

	if err := ctx.Validate(&dto); err != nil {
		return utils.ErrorResponse(ctx, err)
	}

	res, err := c.priorityService.UpdatePriority(reqCtx, id, dto)
	if err != nil {
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

func (c *PriorityController) DeletePriority(ctx echo.Context) error {
	reqCtx := ctx.Request().Context()

	id, err := strconv.ParseUint(ctx.Param("id"), 10, 64)
	if err != nil {
		return utils.ErrorResponse(ctx, fmt.Errorf("invalid prorety ID format: %w", apperrors.ErrBadRequest))
	}

	err = c.priorityService.DeletePriority(reqCtx, id)
	if err != nil {
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
