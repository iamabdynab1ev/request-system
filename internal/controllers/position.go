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

type PositionController struct {
	positionService *services.PositionService
	logger          *zap.Logger
}

func NewPositionController(
	positionService *services.PositionService,
	logger *zap.Logger,
) *PositionController {
	return &PositionController{
		positionService: positionService,
		logger:          logger,
	}
}

func (c *PositionController) GetPositions(ctx echo.Context) error {
	reqCtx := ctx.Request().Context()

	// Используем универсальный парсер фильтров
	filter := utils.ParseFilterFromQuery(ctx.Request().URL.Query())

	// Получаем позиции с пагинацией
	res, total, err := c.positionService.GetPositions(reqCtx, uint64(filter.Limit), uint64(filter.Offset))
	if err != nil {
		c.logger.Error("Ошибка при получении списка должностей", zap.Error(err))
		return utils.ErrorResponse(ctx, err)
	}

	// Возвращаем с учётом total
	return utils.SuccessResponse(
		ctx,
		res,
		"Список должностей успешно получен",
		http.StatusOK,
		total,
	)
}

func (c *PositionController) FindPosition(ctx echo.Context) error {
	reqCtx := ctx.Request().Context()

	id, err := strconv.ParseUint(ctx.Param("id"), 10, 64)
	if err != nil {
		return utils.ErrorResponse(ctx, err)
	}

	res, err := c.positionService.FindPosition(reqCtx, id)
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

func (c *PositionController) CreatePosition(ctx echo.Context) error {
	reqCtx := utils.Ctx(ctx, 5)

	var dto dto.CreatePositionDTO
	if err := ctx.Bind(&dto); err != nil {
		c.logger.Error("неправильный запрос", zap.Error(err))
		return utils.ErrorResponse(ctx, err)
	}

	if err := ctx.Validate(&dto); err != nil {
		c.logger.Error("Ошибка при валидации данных должности: ", zap.Error(err))
		return utils.ErrorResponse(ctx, err)
	}

	res, err := c.positionService.CreatePosition(reqCtx, dto)
	if err != nil {
		c.logger.Error("Ошибка при создании должности: ", zap.Error(err))
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

func (c *PositionController) UpdatePosition(ctx echo.Context) error {
	reqCtx := ctx.Request().Context()

	id, err := strconv.ParseUint(ctx.Param("id"), 10, 64)
	if err != nil {
		c.logger.Error("Ошибка при валидации данных должности: ", zap.Error(err))
		return utils.ErrorResponse(ctx, err)
	}

	var dto dto.UpdatePositionDTO
	if err := ctx.Bind(&dto); err != nil {
		c.logger.Error("Ошибка при валидации данных должности: ", zap.Error(err))
		return utils.ErrorResponse(ctx, err)
	}

	if err := ctx.Validate(&dto); err != nil {
		c.logger.Error("Ошибка при валидации данных должности: ", zap.Error(err))
		return utils.ErrorResponse(ctx, err)
	}

	res, err := c.positionService.UpdatePosition(reqCtx, id, dto)
	if err != nil {
		c.logger.Error("Ошибка при валидации данных должности: ", zap.Error(err))
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

func (c *PositionController) DeletePosition(ctx echo.Context) error {
	reqCtx := ctx.Request().Context()

	id, err := strconv.ParseUint(ctx.Param("id"), 10, 64)
	if err != nil {
		return utils.ErrorResponse(ctx, err)
	}

	err = c.positionService.DeletePosition(reqCtx, id)
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
