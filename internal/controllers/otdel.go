package controllers

import (
	"net/http"
	"strconv"

	"request-system/internal/dto"
	"request-system/internal/services"
	"request-system/pkg/utils"

	"go.uber.org/zap"

	"github.com/labstack/echo/v4"
)

type OtdelController struct {
	otdelService *services.OtdelService
	logger       *zap.Logger
}

func NewOtdelController(
		otdelService *services.OtdelService,
		logger 		*zap.Logger,
) *OtdelController {
	return &OtdelController{
		otdelService: otdelService,
		logger : 		logger,
	}
}

func (c *OtdelController) GetOtdels(ctx echo.Context) error {
	reqCtx := ctx.Request().Context()

	res, err := c.otdelService.GetOtdels(reqCtx, 6, 10)
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


func (c *OtdelController) FindOtdel(ctx echo.Context) error {
	reqCtx := ctx.Request().Context()

	id, err := strconv.ParseUint(ctx.Param("id"), 10, 64)
	if err != nil {
		return utils.ErrorResponse(ctx, err)
	}

	res, err := c.otdelService.FindOtdel(reqCtx, id)
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

func (c *OtdelController) CreateOtdel(ctx echo.Context) error {
	reqCtx := ctx.Request().Context()

	var dto dto.CreateOtdelDTO
	if err := ctx.Bind(&dto); err != nil {
		c.logger.Error("неверный запрос", zap.Error(err))
		return utils.ErrorResponse(ctx, err)
	}

	if err := ctx.Validate(&dto); err != nil {
		c.logger.Error("Ощибка при валидации данных отдел: ", zap.Error(err))
		return utils.ErrorResponse(ctx, err)
	}

	res, err := c.otdelService.CreateOtdel(reqCtx, dto)
	if err != nil {
		c.logger.Error("Ощибка при создание отдел: ", zap.Error(err))
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

func (c *OtdelController) UpdateOtdel(ctx echo.Context) error {
	reqCtx := ctx.Request().Context()

	id, err := strconv.ParseUint(ctx.Param("id"), 10, 64)
	if err != nil {
		return utils.ErrorResponse(ctx, err)
	}

	var dto dto.UpdateOtdelDTO
	if err := ctx.Bind(&dto); err != nil {
		return utils.ErrorResponse(ctx, err)
	}

	if err := ctx.Validate(&dto); err != nil {
		return utils.ErrorResponse(ctx, err)
	}

	res, err := c.otdelService.UpdateOtdel(reqCtx, id, dto)
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

func (c *OtdelController) DeleteOtdel(ctx echo.Context) error {
	reqCtx := ctx.Request().Context()

	id, err := strconv.ParseUint(ctx.Param("id"), 10, 64)
	if err != nil {
		return utils.ErrorResponse(ctx, err)
	}

	err = c.otdelService.DeleteOtdel(reqCtx, id)
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