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
	officeService services.OfficeServiceInterface
	logger        *zap.Logger
}

func NewOfficeController(service services.OfficeServiceInterface, logger *zap.Logger) *OfficeController {
	return &OfficeController{officeService: service, logger: logger}
}

func (c *OfficeController) GetOffices(ctx echo.Context) error {
	// 1. Получаем единый объект фильтра из URL
	filter := utils.ParseFilterFromQuery(ctx.Request().URL.Query())

	// 2. Передаем весь объект filter в сервис
	offices, total, err := c.officeService.GetOffices(ctx.Request().Context(), filter)
	if err != nil {
		c.logger.Error("GetOffices: ошибка при получении списка офисов", zap.Error(err))
		return utils.ErrorResponse(
			ctx,
			apperrors.NewHttpError(
				http.StatusInternalServerError,
				"Не удалось получить список офисов",
				err,
				nil,
			),
			c.logger,
		)
	}

	// 3. Используем стандартный SuccessResponse, который обработает пагинацию
	return utils.SuccessResponse(ctx, offices, "Список офисов успешно получен", http.StatusOK, total)
}

func (c *OfficeController) FindOffice(ctx echo.Context) error {
	id, err := strconv.ParseUint(ctx.Param("id"), 10, 64)
	if err != nil {
		c.logger.Error("FindOffice: некорректный ID офиса", zap.String("id", ctx.Param("id")), zap.Error(err))
		return utils.ErrorResponse(
			ctx,
			apperrors.NewHttpError(
				http.StatusBadRequest,
				"Неверный формат ID офиса",
				err,
				map[string]interface{}{"param": ctx.Param("id")},
			),
			c.logger,
		)
	}

	res, err := c.officeService.FindOffice(ctx.Request().Context(), id)
	if err != nil {
		c.logger.Error("FindOffice: ошибка при поиске офиса", zap.Uint64("id", id), zap.Error(err))
		return utils.ErrorResponse(
			ctx,
			apperrors.NewHttpError(
				http.StatusInternalServerError,
				"Не удалось найти офис",
				err,
				nil,
			),
			c.logger,
		)
	}

	return utils.SuccessResponse(ctx, res, "Офис успешно найден", http.StatusOK)
}

func (c *OfficeController) CreateOffice(ctx echo.Context) error {
	var dto dto.CreateOfficeDTO

	if err := ctx.Bind(&dto); err != nil {
		c.logger.Error("CreateOffice: ошибка парсинга данных", zap.Error(err))
		return utils.ErrorResponse(
			ctx,
			apperrors.NewHttpError(
				http.StatusBadRequest,
				"Неверный формат данных для создания офиса",
				err,
				nil,
			),
			c.logger,
		)
	}

	if err := ctx.Validate(&dto); err != nil {
		c.logger.Error("CreateOffice: ошибка валидации данных", zap.Error(err))
		return utils.ErrorResponse(ctx, err, c.logger)
	}

	res, err := c.officeService.CreateOffice(ctx.Request().Context(), dto)
	if err != nil {
		c.logger.Error("CreateOffice: ошибка при создании офиса в сервисе", zap.Error(err))
		return utils.ErrorResponse(
			ctx,
			apperrors.NewHttpError(
				http.StatusInternalServerError,
				"Не удалось создать офис",
				err,
				nil,
			),
			c.logger,
		)
	}

	return utils.SuccessResponse(ctx, res, "Офис успешно создан", http.StatusCreated)
}

func (c *OfficeController) UpdateOffice(ctx echo.Context) error {
	id, err := strconv.ParseUint(ctx.Param("id"), 10, 64)
	if err != nil {
		c.logger.Error("UpdateOffice: неверный формат ID", zap.String("id", ctx.Param("id")), zap.Error(err))
		return utils.ErrorResponse(
			ctx,
			apperrors.NewHttpError(
				http.StatusBadRequest,
				"Неверный формат ID офиса",
				err,
				map[string]interface{}{"param": ctx.Param("id")},
			),
			c.logger,
		)
	}

	var dto dto.UpdateOfficeDTO
	if err := ctx.Bind(&dto); err != nil {
		c.logger.Error("UpdateOffice: ошибка парсинга данных", zap.Error(err))
		return utils.ErrorResponse(
			ctx,
			apperrors.NewHttpError(
				http.StatusBadRequest,
				"Неверный формат данных для обновления офиса",
				err,
				nil,
			),
			c.logger,
		)
	}

	if err := ctx.Validate(&dto); err != nil {
		c.logger.Error("UpdateOffice: ошибка валидации данных", zap.Error(err))
		return utils.ErrorResponse(ctx, err, c.logger)
	}

	res, err := c.officeService.UpdateOffice(ctx.Request().Context(), id, dto)
	if err != nil {
		c.logger.Error("UpdateOffice: ошибка при обновлении офиса в сервисе", zap.Uint64("id", id), zap.Error(err))
		return utils.ErrorResponse(
			ctx,
			apperrors.NewHttpError(
				http.StatusInternalServerError,
				"Не удалось обновить офис",
				err,
				nil,
			),
			c.logger,
		)
	}

	return utils.SuccessResponse(ctx, res, "Офис успешно обновлен", http.StatusOK)
}

func (c *OfficeController) DeleteOffice(ctx echo.Context) error {
	id, err := strconv.ParseUint(ctx.Param("id"), 10, 64)
	if err != nil {
		c.logger.Error("DeleteOffice: неверный формат ID", zap.String("id", ctx.Param("id")), zap.Error(err))
		return utils.ErrorResponse(
			ctx,
			apperrors.NewHttpError(
				http.StatusBadRequest,
				"Неверный формат ID офиса",
				err,
				map[string]interface{}{"param": ctx.Param("id")},
			),
			c.logger,
		)
	}

	if err := c.officeService.DeleteOffice(ctx.Request().Context(), id); err != nil {
		c.logger.Error("DeleteOffice: ошибка при удалении офиса", zap.Uint64("id", id), zap.Error(err))
		return utils.ErrorResponse(
			ctx,
			apperrors.NewHttpError(
				http.StatusInternalServerError,
				"Не удалось удалить офис",
				err,
				nil,
			),
			c.logger,
		)
	}

	return utils.SuccessResponse(ctx, nil, "Офис успешно удален", http.StatusOK)
}
