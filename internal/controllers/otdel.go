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

type OtdelController struct {
	otdelService services.OtdelServiceInterface
	logger       *zap.Logger
}

func NewOtdelController(service services.OtdelServiceInterface, logger *zap.Logger) *OtdelController {
	return &OtdelController{otdelService: service, logger: logger}
}

func (c *OtdelController) GetOtdels(ctx echo.Context) error {
	filter := utils.ParseFilterFromQuery(ctx.Request().URL.Query())
	res, total, err := c.otdelService.GetOtdels(ctx.Request().Context(), filter)
	if err != nil {
		c.logger.Error("Ошибка получения списка отделов", zap.Error(err))
		return utils.ErrorResponse(ctx, err)
	}
	return utils.SuccessResponse(ctx, res, "Успешно", http.StatusOK, total)
}

func (c *OtdelController) FindOtdel(ctx echo.Context) error {
	id, err := strconv.ParseUint(ctx.Param("id"), 10, 64)
	if err != nil {
		return utils.ErrorResponse(ctx, apperrors.NewBadRequestError("Неверный формат ID"))
	}
	res, err := c.otdelService.FindOtdel(ctx.Request().Context(), id)
	if err != nil {
		c.logger.Error("Ошибка поиска отдела", zap.Uint64("id", id), zap.Error(err))
		return utils.ErrorResponse(ctx, err)
	}
	return utils.SuccessResponse(ctx, res, "Успешно", http.StatusOK)
}

func (c *OtdelController) CreateOtdel(ctx echo.Context) error {
	var dto dto.CreateOtdelDTO
	if err := ctx.Bind(&dto); err != nil {
		return utils.ErrorResponse(ctx, apperrors.NewBadRequestError("Неверный формат данных"))
	}
	if err := ctx.Validate(&dto); err != nil {
		return utils.ErrorResponse(ctx, err)
	}

	res, err := c.otdelService.CreateOtdel(ctx.Request().Context(), dto)
	if err != nil {
		c.logger.Error("Ошибка создания отдела", zap.Error(err))
		return utils.ErrorResponse(ctx, err)
	}
	return utils.SuccessResponse(ctx, res, "Успешно создан", http.StatusCreated)
}

func (c *OtdelController) UpdateOtdel(ctx echo.Context) error {
	id, err := strconv.ParseUint(ctx.Param("id"), 10, 64)
	if err != nil {
		return utils.ErrorResponse(ctx, apperrors.NewBadRequestError("Неверный формат ID"))
	}

	var dto dto.UpdateOtdelDTO
	if err := ctx.Bind(&dto); err != nil {
		return utils.ErrorResponse(ctx, apperrors.NewBadRequestError("Неверный формат данных"))
	}
	if err := ctx.Validate(&dto); err != nil {
		return utils.ErrorResponse(ctx, err)
	}

	res, err := c.otdelService.UpdateOtdel(ctx.Request().Context(), id, dto)
	if err != nil {
		c.logger.Error("Ошибка обновления отдела", zap.Uint64("id", id), zap.Error(err))
		return utils.ErrorResponse(ctx, err)
	}
	return utils.SuccessResponse(ctx, res, "Успешно обновлен", http.StatusOK)
}

func (c *OtdelController) DeleteOtdel(ctx echo.Context) error {
	id, err := strconv.ParseUint(ctx.Param("id"), 10, 64)
	if err != nil {
		return utils.ErrorResponse(ctx, apperrors.NewBadRequestError("Неверный формат ID"))
	}

	if err := c.otdelService.DeleteOtdel(ctx.Request().Context(), id); err != nil {
		c.logger.Error("Ошибка удаления отдела", zap.Uint64("id", id), zap.Error(err))
		return utils.ErrorResponse(ctx, err)
	}
	return utils.SuccessResponse(ctx, nil, "Успешно удален", http.StatusOK)
}
