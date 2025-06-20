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

type EquipmentTypeController struct {
	equipmentTypeService *services.EquipmentTypeService
	logger               *zap.Logger
}

func NewEquipmentTypeController(
	equipmentTypeService *services.EquipmentTypeService,
	logger *zap.Logger,
) *EquipmentTypeController {
	return &EquipmentTypeController{
		equipmentTypeService: equipmentTypeService,
		logger:               logger,
	}
}

func (c *EquipmentTypeController) GetEquipmentTypes(ctx echo.Context) error {
	reqCtx := ctx.Request().Context()

	res, err := c.equipmentTypeService.GetEquipmentTypes(reqCtx)
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

func (c *EquipmentTypeController) FindEquipmentType(ctx echo.Context) error {
	reqCtx := ctx.Request().Context()

	id, err := strconv.ParseUint(ctx.Param("id"), 10, 64)
	if err != nil {
		return utils.ErrorResponse(ctx, err)
	}

	res, err := c.equipmentTypeService.FindEquipmentType(reqCtx, id)
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

func (c *EquipmentTypeController) CreateEquipmentType(ctx echo.Context) error {
	reqCtx := ctx.Request().Context()

	var dto dto.CreateEquipmentTypeDTO
	if err := ctx.Bind(&dto); err != nil {
		c.logger.Error("неверный запрос", zap.Error(err))
		return utils.ErrorResponse(ctx, err)
	}

	if err := ctx.Validate(&dto); err != nil {
		c.logger.Error("Ощибка при валидации данных оборудования: ", zap.Error(err))
		return utils.ErrorResponse(ctx, err)
	}

	res, err := c.equipmentTypeService.CreateEquipmentType(reqCtx, dto)
	if err != nil {
		c.logger.Error("Ощибка при создание оборудования: ", zap.Error(err))
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

func (c *EquipmentTypeController) UpdateEquipmentType(ctx echo.Context) error {
	reqCtx := ctx.Request().Context()

	id, err := strconv.ParseUint(ctx.Param("id"), 10, 64)
	if err != nil {
		return utils.ErrorResponse(ctx, err)
	}

	var dto dto.UpdateEquipmentTypeDTO
	if err := ctx.Bind(&dto); err != nil {
		return utils.ErrorResponse(ctx, err)
	}

	if err := ctx.Validate(&dto); err != nil {
		return utils.ErrorResponse(ctx, err)
	}

	res, err := c.equipmentTypeService.UpdateEquipmentType(reqCtx, id, dto)
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

func (c *EquipmentTypeController) DeleteEquipmentType(ctx echo.Context) error {
	reqCtx := ctx.Request().Context()

	id, err := strconv.ParseUint(ctx.Param("id"), 10, 64)
	if err != nil {
		return utils.ErrorResponse(ctx, err)
	}

	err = c.equipmentTypeService.DeleteEquipmentType(reqCtx, id)
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
