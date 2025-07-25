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

type EquipmentController struct {
	equipmentService *services.EquipmentService
	logger           *zap.Logger
}

func NewEquipmentController(
	equipmentService *services.EquipmentService,
	logger *zap.Logger,
) *EquipmentController {
	return &EquipmentController{
		equipmentService: equipmentService,
		logger:           logger,
	}
}

func (c *EquipmentController) GetEquipments(ctx echo.Context) error {
	reqCtx := utils.Ctx(ctx, 5) 

	filter := utils.ParseFilterFromQuery(ctx.Request().URL.Query())

	res, total, err := c.equipmentService.GetEquipments(reqCtx, uint64(filter.Limit), uint64(filter.Offset))

	if err != nil {
		return utils.ErrorResponse(ctx, err)
	}

	return utils.SuccessResponse(ctx, res, "Комментарии успешно получены", http.StatusOK, total)
}

func (c *EquipmentController) FindEquipment(ctx echo.Context) error {
	reqCtx := ctx.Request().Context()

	id, err := strconv.ParseUint(ctx.Param("id"), 10, 64)
	if err != nil {
		return utils.ErrorResponse(ctx, err)
	}

	res, err := c.equipmentService.FindEquipment(reqCtx, id)
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

func (c *EquipmentController) CreateEquipment(ctx echo.Context) error {
	reqCtx := utils.Ctx(ctx, 5)

	var dto dto.CreateEquipmentDTO
	if err := ctx.Bind(&dto); err != nil {
		c.logger.Error("неправильный запрос", zap.Error(err))
		return utils.ErrorResponse(ctx, err)
	}

	if err := ctx.Validate(&dto); err != nil {
		c.logger.Error("Ощибка при валидации данных оборудования: ", zap.Error(err))
		return utils.ErrorResponse(ctx, err)
	}

	res, err := c.equipmentService.CreateEquipment(reqCtx, dto)
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

func (c *EquipmentController) UpdateEquipment(ctx echo.Context) error {
	reqCtx := ctx.Request().Context()

	id, err := strconv.ParseUint(ctx.Param("id"), 10, 64)
	if err != nil {
		return utils.ErrorResponse(ctx, err)
	}

	var dto dto.UpdateEquipmentDTO
	if err := ctx.Bind(&dto); err != nil {
		return utils.ErrorResponse(ctx, err)
	}

	if err := ctx.Validate(&dto); err != nil {
		return utils.ErrorResponse(ctx, err)
	}

	res, err := c.equipmentService.UpdateEquipment(reqCtx, id, dto)
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

func (c *EquipmentController) DeleteEquipment(ctx echo.Context) error {
	reqCtx := ctx.Request().Context()

	id, err := strconv.ParseUint(ctx.Param("id"), 10, 64)
	if err != nil {
		return utils.ErrorResponse(ctx, err)
	}

	err = c.equipmentService.DeleteEquipment(reqCtx, id)
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
