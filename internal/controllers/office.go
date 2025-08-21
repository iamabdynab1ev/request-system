// Файл: internal/controllers/office.go
// СКОПИРУЙТЕ И ПОЛНОСТЬЮ ЗАМЕНИТЕ СОДЕРЖИМОЕ

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

type OfficeController struct {
	officeService *services.OfficeService
	logger        *zap.Logger
}

func NewOfficeController(service *services.OfficeService, logger *zap.Logger) *OfficeController {
	return &OfficeController{officeService: service, logger: logger}
}

func (c *OfficeController) GetOffices(ctx echo.Context) error {
	// 1. Получаем единый объект фильтра из URL
	filter := utils.ParseFilterFromQuery(ctx.Request().URL.Query())

	// 2. Передаем ВЕСЬ объект filter в сервис
	offices, total, err := c.officeService.GetOffices(ctx.Request().Context(), filter)
	if err != nil {
		c.logger.Error("Ошибка при получении списка офисов", zap.Error(err))
		return utils.ErrorResponse(ctx, err)
	}

	// 3. Используем стандартный SuccessResponse, который обработает пагинацию
	return utils.SuccessResponse(ctx, offices, "Список офисов успешно получен", http.StatusOK, total)
}

func (c *OfficeController) FindOffice(ctx echo.Context) error {
	id, err := strconv.ParseUint(ctx.Param("id"), 10, 64)
	if err != nil {
		return utils.ErrorResponse(ctx, apperrors.NewBadRequestError("Неверный формат ID офиса"))
	}
	res, err := c.officeService.FindOffice(ctx.Request().Context(), id)
	if err != nil {
		c.logger.Error("Ошибка при поиске офиса", zap.Uint64("id", id), zap.Error(err))
		return utils.ErrorResponse(ctx, err)
	}
	return utils.SuccessResponse(ctx, res, "Офис успешно найден", http.StatusOK)
}

func (c *OfficeController) CreateOffice(ctx echo.Context) error {
	var dto dto.CreateOfficeDTO
	if err := ctx.Bind(&dto); err != nil {
		return utils.ErrorResponse(ctx, apperrors.NewBadRequestError("Неверный формат данных"))
	}
	if err := ctx.Validate(&dto); err != nil {
		return utils.ErrorResponse(ctx, err)
	}
	res, err := c.officeService.CreateOffice(ctx.Request().Context(), dto)
	if err != nil {
		c.logger.Error("Ошибка при создании офиса", zap.Error(err))
		return utils.ErrorResponse(ctx, err)
	}
	return utils.SuccessResponse(ctx, res, "Офис успешно создан", http.StatusCreated)
}

func (c *OfficeController) UpdateOffice(ctx echo.Context) error {
	id, err := strconv.ParseUint(ctx.Param("id"), 10, 64)
	if err != nil {
		return utils.ErrorResponse(ctx, apperrors.NewBadRequestError("Неверный формат ID офиса"))
	}
	var dto dto.UpdateOfficeDTO
	if err := ctx.Bind(&dto); err != nil {
		return utils.ErrorResponse(ctx, apperrors.NewBadRequestError("Неверный формат данных"))
	}
	if err := ctx.Validate(&dto); err != nil {
		return utils.ErrorResponse(ctx, err)
	}
	res, err := c.officeService.UpdateOffice(ctx.Request().Context(), id, dto)
	if err != nil {
		c.logger.Error("Ошибка при обновлении офиса", zap.Uint64("id", id), zap.Error(err))
		return utils.ErrorResponse(ctx, err)
	}
	return utils.SuccessResponse(ctx, res, "Офис успешно обновлен", http.StatusOK)
}

func (c *OfficeController) DeleteOffice(ctx echo.Context) error {
	id, err := strconv.ParseUint(ctx.Param("id"), 10, 64)
	if err != nil {
		return utils.ErrorResponse(ctx, apperrors.NewBadRequestError("Неверный формат ID офиса"))
	}
	if err := c.officeService.DeleteOffice(ctx.Request().Context(), id); err != nil {
		c.logger.Error("Ошибка при удалении офиса", zap.Uint64("id", id), zap.Error(err))
		return utils.ErrorResponse(ctx, err)
	}
	return utils.SuccessResponse(ctx, nil, "Офис успешно удален", http.StatusOK)
}
