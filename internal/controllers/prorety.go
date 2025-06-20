package controllers

import (
	"fmt"
	"net/http"
	"strconv"

	"request-system/internal/dto"
	"request-system/internal/services"
	"request-system/pkg/utils"

	"go.uber.org/zap"

	"github.com/labstack/echo/v4"
)

type ProretyController struct {
	proretyService *services.ProretyService
	logger         *zap.Logger
}

func NewProretyController(proretyService *services.ProretyService,
	logger *zap.Logger,
) *ProretyController {
	return &ProretyController{
		proretyService: proretyService,
		logger:         logger,
	}
}

func (c *ProretyController) GetProreties(ctx echo.Context) error {
	reqCtx := ctx.Request().Context()

	res, err := c.proretyService.GetProreties(reqCtx, 6, 10)
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

func (c *ProretyController) FindProrety(ctx echo.Context) error {
	reqCtx := ctx.Request().Context()

	id, err := strconv.ParseUint(ctx.Param("id"), 10, 64)
	if err != nil {
		return utils.ErrorResponse(ctx, fmt.Errorf("invalid prorety ID format: %w", utils.ErrorBadRequest))
	}

	res, err := c.proretyService.FindProrety(reqCtx, id)
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

func (c *ProretyController) CreateProrety(ctx echo.Context) error {
	reqCtx := ctx.Request().Context()

	var dto dto.CreateProretyDTO
	if err := ctx.Bind(&dto); err != nil {
		c.logger.Error("неверный запрос", zap.Error(err))
		return utils.ErrorResponse(ctx, fmt.Errorf("request binding failed: %w", utils.ErrorBadRequest))
	}

	if err := ctx.Validate(&dto); err != nil {
		c.logger.Error("Ощибка при валидации данных: ", zap.Error(err))
		return utils.ErrorResponse(ctx, err)
	}

	res, err := c.proretyService.CreateProrety(reqCtx, dto)
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

func (c *ProretyController) UpdateProrety(ctx echo.Context) error {
	reqCtx := ctx.Request().Context()

	id, err := strconv.ParseUint(ctx.Param("id"), 10, 64)
	if err != nil {
		return utils.ErrorResponse(ctx, fmt.Errorf("invalid prorety ID format: %w", utils.ErrorBadRequest))
	}

	var dto dto.UpdateProretyDTO
	if err := ctx.Bind(&dto); err != nil {
		return utils.ErrorResponse(ctx, fmt.Errorf("request binding failed: %w", utils.ErrorBadRequest))
	}

	if err := ctx.Validate(&dto); err != nil {
		return utils.ErrorResponse(ctx, err)
	}

	res, err := c.proretyService.UpdateProrety(reqCtx, id, dto)
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

func (c *ProretyController) DeleteProrety(ctx echo.Context) error {
	reqCtx := ctx.Request().Context()

	id, err := strconv.ParseUint(ctx.Param("id"), 10, 64)
	if err != nil {
		return utils.ErrorResponse(ctx, fmt.Errorf("invalid prorety ID format: %w", utils.ErrorBadRequest))
	}

	err = c.proretyService.DeleteProrety(reqCtx, id)
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
