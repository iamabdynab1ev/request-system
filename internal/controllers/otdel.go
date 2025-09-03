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

type OtdelController struct {
	otdelService services.OtdelServiceInterface
	logger       *zap.Logger
}

func NewOtdelController(service services.OtdelServiceInterface, logger *zap.Logger) *OtdelController {
	return &OtdelController{otdelService: service, logger: logger}
}

func (c *OtdelController) GetOtdels(ctx echo.Context) error {
	reqCtx := ctx.Request().Context()
	filter := utils.ParseFilterFromQuery(ctx.Request().URL.Query())

	res, total, err := c.otdelService.GetOtdels(reqCtx, filter)
	if err != nil {
		c.logger.Error("GetOtdels: ошибка при получении списка отделов", zap.Error(err))
		return utils.ErrorResponse(ctx,
			apperrors.NewHttpError(
				http.StatusInternalServerError,
				"Не удалось получить список отделов",
				err,
				nil,
			),
			c.logger,
		)
	}

	return utils.SuccessResponse(
		ctx,
		res,
		"Список отделов успешно получен",
		http.StatusOK,
		total,
	)
}

func (c *OtdelController) FindOtdel(ctx echo.Context) error {
	id, err := strconv.ParseUint(ctx.Param("id"), 10, 64)
	if err != nil {
		c.logger.Error("FindOtdel: неверный формат ID", zap.String("id", ctx.Param("id")), zap.Error(err))
		return utils.ErrorResponse(ctx,
			apperrors.NewHttpError(
				http.StatusBadRequest,
				"Неверный формат ID отдела",
				err,
				map[string]interface{}{"param": ctx.Param("id")},
			),
			c.logger,
		)
	}
	res, err := c.otdelService.FindOtdel(ctx.Request().Context(), id)
	if err != nil {
		c.logger.Error("FindOtdel: ошибка при поиске отдела", zap.Uint64("id", id), zap.Error(err))
		return utils.ErrorResponse(ctx,
			apperrors.NewHttpError(
				http.StatusInternalServerError,
				"Не удалось найти отдел",
				err,
				nil,
			),
			c.logger,
		)
	}
	return utils.SuccessResponse(ctx, res, "Успешно", http.StatusOK)
}

func (c *OtdelController) CreateOtdel(ctx echo.Context) error {
	var dto dto.CreateOtdelDTO
	if err := ctx.Bind(&dto); err != nil {
		c.logger.Error("CreateOtdel: неверный формат запроса", zap.Error(err))
		return utils.ErrorResponse(ctx,
			apperrors.NewHttpError(
				http.StatusBadRequest,
				"Неверный формат запроса",
				err,
				nil,
			),
			c.logger,
		)
	}
	if err := ctx.Validate(&dto); err != nil {
		c.logger.Error("CreateOtdel: ошибка валидации данных", zap.Error(err))
		return utils.ErrorResponse(ctx, err, c.logger)
	}

	res, err := c.otdelService.CreateOtdel(ctx.Request().Context(), dto)
	if err != nil {
		c.logger.Error("CreateOtdel: ошибка при создании отдела", zap.Error(err))
		return utils.ErrorResponse(ctx,
			apperrors.NewHttpError(
				http.StatusInternalServerError,
				"Не удалось создать отдел",
				err,
				nil,
			),
			c.logger,
		)
	}
	return utils.SuccessResponse(ctx, res, "Успешно создан", http.StatusCreated)
}

func (c *OtdelController) UpdateOtdel(ctx echo.Context) error {
	id, err := strconv.ParseUint(ctx.Param("id"), 10, 64)
	if err != nil {
		c.logger.Error("UpdateOtdel: неверный формат ID", zap.String("id", ctx.Param("id")), zap.Error(err))
		return utils.ErrorResponse(ctx,
			apperrors.NewHttpError(
				http.StatusBadRequest,
				"Неверный формат ID отдела",
				err,
				map[string]interface{}{"param": ctx.Param("id")},
			),
			c.logger,
		)
	}

	var dto dto.UpdateOtdelDTO
	if err := ctx.Bind(&dto); err != nil {
		c.logger.Error("UpdateOtdel: неверный формат запроса", zap.Error(err))
		return utils.ErrorResponse(ctx,
			apperrors.NewHttpError(
				http.StatusBadRequest,
				"Неверный формат запроса",
				err,
				nil,
			),
			c.logger,
		)
	}
	if err := ctx.Validate(&dto); err != nil {
		c.logger.Error("UpdateOtdel: ошибка валидации данных", zap.Error(err))
		return utils.ErrorResponse(ctx, err, c.logger)
	}

	res, err := c.otdelService.UpdateOtdel(ctx.Request().Context(), id, dto)
	if err != nil {
		c.logger.Error("UpdateOtdel: ошибка при обновлении отдела", zap.Uint64("id", id), zap.Error(err))
		return utils.ErrorResponse(ctx,
			apperrors.NewHttpError(
				http.StatusInternalServerError,
				"Не удалось обновить отдел",
				err,
				nil,
			),
			c.logger,
		)
	}
	return utils.SuccessResponse(ctx, res, "Успешно обновлен", http.StatusOK)
}

func (c *OtdelController) DeleteOtdel(ctx echo.Context) error {
	id, err := strconv.ParseUint(ctx.Param("id"), 10, 64)
	if err != nil {
		c.logger.Error("DeleteOtdel: неверный формат ID", zap.String("id", ctx.Param("id")), zap.Error(err))
		return utils.ErrorResponse(ctx,
			apperrors.NewHttpError(
				http.StatusBadRequest,
				"Неверный формат ID отдела",
				err,
				map[string]interface{}{"param": ctx.Param("id")},
			),
			c.logger,
		)
	}

	if err := c.otdelService.DeleteOtdel(ctx.Request().Context(), id); err != nil {
		c.logger.Error("DeleteOtdel: ошибка при удалении отдела", zap.Uint64("id", id), zap.Error(err))
		return utils.ErrorResponse(ctx,
			apperrors.NewHttpError(
				http.StatusInternalServerError,
				"Не удалось удалить отдел",
				err,
				nil,
			),
			c.logger,
		)
	}
	return utils.SuccessResponse(ctx, nil, "Успешно удален", http.StatusOK)
}
