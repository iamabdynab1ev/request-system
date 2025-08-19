// Файл: internal/controllers/office.go
// СКОПИРУЙТЕ И ПОЛНОСТЬЮ ЗАМЕНИТЕ СОДЕРЖИМОЕ

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
	filter := utils.ParseFilterFromQuery(ctx.QueryParams())

	offices, total, err := c.officeService.GetOffices(reqCtx, uint64(filter.Limit), uint64(filter.Offset))
	if err != nil {
		c.logger.Error("Ошибка при получении списка офисов", zap.Error(err))
		return utils.ErrorResponse(ctx, err)
	}
	return utils.SuccessResponse(ctx, offices, "Список офисов успешно получен", http.StatusOK, total)
}

func (c *OfficeController) FindOffice(ctx echo.Context) error {
	reqCtx := ctx.Request().Context()
	id, err := strconv.ParseUint(ctx.Param("id"), 10, 64)
	if err != nil {
		return utils.ErrorResponse(ctx, apperrors.NewBadRequestError("Некорректный ID офиса"))
	}
	res, err := c.officeService.FindOffice(reqCtx, id)
	if err != nil {
		c.logger.Error("Ошибка при поиске офиса", zap.Error(err), zap.Uint64("id", id))
		return utils.ErrorResponse(ctx, err)
	}
	return utils.SuccessResponse(ctx, res, "Офис успешно найден", http.StatusOK)
}

func (c *OfficeController) CreateOffice(ctx echo.Context) error {
	reqCtx := ctx.Request().Context()
	var dto dto.CreateOfficeDTO
	if err := ctx.Bind(&dto); err != nil {
		return utils.ErrorResponse(ctx, apperrors.NewBadRequestError("Неверный формат данных"))
	}
	if err := ctx.Validate(&dto); err != nil {
		return utils.ErrorResponse(ctx, apperrors.NewBadRequestError(err.Error()))
	}
	res, err := c.officeService.CreateOffice(reqCtx, dto)
	if err != nil {
		c.logger.Error("Ошибка при создании офиса", zap.Error(err))
		return utils.ErrorResponse(ctx, err)
	}
	return utils.SuccessResponse(ctx, res, "Офис успешно создан", http.StatusCreated)
}

func (c *OfficeController) UpdateOffice(ctx echo.Context) error {
	reqCtx := ctx.Request().Context()
	id, err := strconv.ParseUint(ctx.Param("id"), 10, 64)
	if err != nil {
		return utils.ErrorResponse(ctx, apperrors.NewBadRequestError("Некорректный ID офиса"))
	}
	var dto dto.UpdateOfficeDTO
	if err := ctx.Bind(&dto); err != nil {
		return utils.ErrorResponse(ctx, apperrors.NewBadRequestError("Неверный формат данных"))
	}
	if err := ctx.Validate(&dto); err != nil {
		return utils.ErrorResponse(ctx, apperrors.NewBadRequestError(err.Error()))
	}
	res, err := c.officeService.UpdateOffice(reqCtx, id, dto)
	if err != nil {
		c.logger.Error("Ошибка при обновлении офиса", zap.Error(err))
		return utils.ErrorResponse(ctx, err)
	}
	return utils.SuccessResponse(ctx, res, "Офис успешно обновлен", http.StatusOK)
}

func (c *OfficeController) DeleteOffice(ctx echo.Context) error {
	reqCtx := ctx.Request().Context()
	id, err := strconv.ParseUint(ctx.Param("id"), 10, 64)
	if err != nil {
		return utils.ErrorResponse(ctx, apperrors.NewBadRequestError("Некорректный ID офиса"))
	}
	err = c.officeService.DeleteOffice(reqCtx, id)
	if err != nil {
		c.logger.Error("Ошибка при удалении офиса", zap.Error(err), zap.Uint64("id", id))
		return utils.ErrorResponse(ctx, err)
	}
	return ctx.NoContent(http.StatusNoContent)
}
