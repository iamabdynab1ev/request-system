package controllers

import (
	"net/http"
	"strconv"

	"request-system/internal/dto"
	"request-system/internal/services"
	apperrors "request-system/pkg/errors"
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
	filter := utils.ParseFilterFromQuery(ctx.Request().URL.Query())

	res, total, err := c.positionService.GetPositions(reqCtx, uint64(filter.Limit), uint64(filter.Offset))
	if err != nil {
		c.logger.Error("GetPositions: ошибка при получении списка должностей", zap.Error(err))
		return utils.ErrorResponse(ctx, apperrors.NewHttpError(
			http.StatusInternalServerError,
			"Не удалось получить список должностей",
			err,
			nil,
		), c.logger)
	}

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
		c.logger.Error("FindPosition: некорректный ID", zap.String("id", ctx.Param("id")), zap.Error(err))
		return utils.ErrorResponse(ctx, apperrors.NewHttpError(
			http.StatusBadRequest,
			"Неверный формат ID должности",
			err,
			nil,
		), c.logger)
	}

	res, err := c.positionService.FindPosition(reqCtx, id)
	if err != nil {
		c.logger.Error("FindPosition: ошибка при поиске должности", zap.Uint64("id", id), zap.Error(err))
		return utils.ErrorResponse(ctx, apperrors.NewHttpError(
			http.StatusInternalServerError,
			"Не удалось найти должность",
			err,
			nil,
		), c.logger)
	}

	return utils.SuccessResponse(
		ctx,
		res,
		"Должность успешно найдена",
		http.StatusOK,
	)
}

func (c *PositionController) CreatePosition(ctx echo.Context) error {
	reqCtx := utils.Ctx(ctx, 5)

	var dto dto.CreatePositionDTO
	if err := ctx.Bind(&dto); err != nil {
		c.logger.Error("CreatePosition: неверный формат запроса", zap.Error(err))
		return utils.ErrorResponse(ctx,
			apperrors.NewHttpError(
				http.StatusBadRequest,
				"Неверный формат запроса для создания должности",
				err,
				nil,
			),
			c.logger,
		)
	}

	if err := ctx.Validate(&dto); err != nil {
		c.logger.Error("CreatePosition: ошибка валидации DTO", zap.Error(err))
		return utils.ErrorResponse(ctx,
			apperrors.NewHttpError(
				http.StatusBadRequest,
				"Ошибка валидации данных должности",
				err,
				nil,
			),
			c.logger,
		)
	}

	res, err := c.positionService.CreatePosition(reqCtx, dto)
	if err != nil {
		c.logger.Error("CreatePosition: ошибка из сервиса", zap.Error(err))
		return utils.ErrorResponse(ctx,
			apperrors.NewHttpError(
				http.StatusInternalServerError,
				"Не удалось создать должность",
				err,
				nil,
			),
			c.logger,
		)
	}

	return utils.SuccessResponse(ctx, res, "Должность успешно создана", http.StatusCreated)
}

func (c *PositionController) UpdatePosition(ctx echo.Context) error {
	reqCtx := ctx.Request().Context()

	// Парсим ID
	id, err := strconv.ParseUint(ctx.Param("id"), 10, 64)
	if err != nil {
		c.logger.Error("UpdatePosition: неверный ID", zap.String("id", ctx.Param("id")), zap.Error(err))
		return utils.ErrorResponse(ctx,
			apperrors.NewHttpError(
				http.StatusBadRequest,
				"Неверный формат ID должности",
				err,
				map[string]interface{}{"param": ctx.Param("id")},
			),
			c.logger,
		)
	}

	var dto dto.UpdatePositionDTO
	if err := ctx.Bind(&dto); err != nil {
		c.logger.Error("UpdatePosition: ошибка bind", zap.Error(err))
		return utils.ErrorResponse(ctx,
			apperrors.NewHttpError(
				http.StatusBadRequest,
				"Неверный формат запроса для обновления должности",
				err,
				nil,
			),
			c.logger,
		)
	}

	if err := ctx.Validate(&dto); err != nil {
		c.logger.Error("UpdatePosition: ошибка валидации DTO", zap.Error(err))
		return utils.ErrorResponse(ctx,
			apperrors.NewHttpError(
				http.StatusBadRequest,
				"Ошибка валидации данных должности",
				err,
				nil,
			),
			c.logger,
		)
	}

	res, err := c.positionService.UpdatePosition(reqCtx, id, dto)
	if err != nil {
		c.logger.Error("UpdatePosition: ошибка из сервиса", zap.Uint64("id", id), zap.Error(err))
		return utils.ErrorResponse(ctx,
			apperrors.NewHttpError(
				http.StatusInternalServerError,
				"Не удалось обновить должность",
				err,
				nil,
			),
			c.logger,
		)
	}

	return utils.SuccessResponse(ctx, res, "Должность успешно обновлена", http.StatusOK)
}

func (c *PositionController) DeletePosition(ctx echo.Context) error {
	reqCtx := ctx.Request().Context()

	// Парсим ID
	id, err := strconv.ParseUint(ctx.Param("id"), 10, 64)
	if err != nil {
		c.logger.Error("DeletePosition: неверный ID", zap.String("id", ctx.Param("id")), zap.Error(err))
		return utils.ErrorResponse(ctx,
			apperrors.NewHttpError(
				http.StatusBadRequest,
				"Неверный формат ID должности",
				err,
				map[string]interface{}{"param": ctx.Param("id")},
			),
			c.logger,
		)
	}

	// Удаляем через сервис
	if err := c.positionService.DeletePosition(reqCtx, id); err != nil {
		c.logger.Error("DeletePosition: ошибка из сервиса", zap.Uint64("id", id), zap.Error(err))
		return utils.ErrorResponse(ctx,
			apperrors.NewHttpError(
				http.StatusInternalServerError,
				"Не удалось удалить должность",
				err,
				nil,
			),
			c.logger,
		)
	}

	// Успех
	return utils.SuccessResponse(
		ctx,
		struct{}{},
		"Должность успешно удалена",
		http.StatusOK,
	)
}
