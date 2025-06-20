package controllers

import (
	"net/http"
	"strconv"

	"request-system/internal/dto"
	"request-system/internal/services"
	"request-system/pkg/utils"

	"github.com/labstack/echo/v4"
	"go.uber.org/zap"
)

type OfficeController struct {
	officeService *services.OfficeService
	logger        *zap.Logger
}

func NewOfficeController(
	officeService *services.OfficeService,
	logger *zap.Logger,
) *OfficeController {
	return &OfficeController{
		officeService: officeService,
		logger:        logger,
	}
}

func (c *OfficeController) GetOffices(ctx echo.Context) error {
	reqCtx := ctx.Request().Context()

	res, err := c.officeService.GetOffices(reqCtx, 6, 10)
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

func (c *OfficeController) FindOffice(ctx echo.Context) error {
	reqCtx := ctx.Request().Context()

	id, err := strconv.ParseUint(ctx.Param("id"), 10, 64)
	if err != nil {
		return utils.ErrorResponse(ctx, err)
	}

	res, err := c.officeService.FindOffice(reqCtx, id)
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

func (c *OfficeController) CreateOffice(ctx echo.Context) error {
	reqCtx := ctx.Request().Context()

	var dto dto.CreateOfficeDTO
	if err := ctx.Bind(&dto); err != nil {
		c.logger.Error("неверный запрос", zap.Error(err))
		return utils.ErrorResponse(ctx, err)
	}
	if err := ctx.Validate(&dto); err != nil {
		c.logger.Error("Ощибка при валидации данных офиса: ", zap.Error(err))
		return utils.ErrorResponse(ctx, err)
	}

	res, err := c.officeService.CreateOffice(reqCtx, dto)
	if err != nil {
		c.logger.Error("Ощибка при создание офис: ", zap.Error(err))
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

func (c *OfficeController) UpdateOffice(ctx echo.Context) error {
	reqCtx := ctx.Request().Context()

	id, err := strconv.ParseUint(ctx.Param("id"), 10, 64)
	if err != nil {
		c.logger.Error("неверный запрос", zap.Error(err))
		return utils.ErrorResponse(ctx, err)
	}

	var dto dto.UpdateOfficeDTO
	if err := ctx.Bind(&dto); err != nil {
		c.logger.Error("неверный запрос", zap.Error(err))
		return utils.ErrorResponse(ctx, err)
	}

	res, err := c.officeService.UpdateOffice(reqCtx, id, dto)
	if err != nil {
		c.logger.Error("Ощибка при обновление офиса: ", zap.Error(err))
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

func (c *OfficeController) DeleteOffice(ctx echo.Context) error {
	reqCtx := ctx.Request().Context()

	id, err := strconv.ParseUint(ctx.Param("id"), 10, 64)
	if err != nil {
		c.logger.Error("неверный запрос", zap.Error(err))
		return utils.ErrorResponse(ctx, err)
	}

	err = c.officeService.DeleteOffice(reqCtx, id)
	if err != nil {
		c.logger.Error("Ощибка при удаление офиса: ", zap.Error(err))
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
