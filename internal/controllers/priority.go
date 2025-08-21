// Файл: internal/controllers/priority_controller.go
package controllers

import (
	"encoding/json"
	"net/http"
	"strconv"

	"request-system/internal/dto"
	"request-system/internal/services"
	apperrors "request-system/pkg/errors"
	"request-system/pkg/utils"

	"github.com/labstack/echo/v4"
	"go.uber.org/zap"
)

type PriorityController struct {
	priorityService services.PriorityServiceInterface
	logger          *zap.Logger
}

func NewPriorityController(priorityService services.PriorityServiceInterface, logger *zap.Logger) *PriorityController {
	return &PriorityController{priorityService: priorityService, logger: logger}
}

func (c *PriorityController) GetPriorities(ctx echo.Context) error {
	reqCtx := ctx.Request().Context()
	filter := utils.ParseFilterFromQuery(ctx.Request().URL.Query())

	res, err := c.priorityService.GetPriorities(reqCtx, uint64(filter.Limit), uint64(filter.Offset), filter.Search)
	if err != nil {
		c.logger.Error("Ошибка получения списка приоритетов", zap.Error(err))
		return utils.ErrorResponse(ctx, err)
	}

	return utils.SuccessResponse(ctx, res.List, "Список приоритетов успешно получен", http.StatusOK, res.Pagination.TotalCount)
}

func (c *PriorityController) FindPriority(ctx echo.Context) error {
	reqCtx := ctx.Request().Context()
	id, err := strconv.ParseUint(ctx.Param("id"), 10, 64)
	if err != nil {
		return utils.ErrorResponse(ctx, apperrors.NewHttpError(http.StatusBadRequest, "Неверный ID", err))
	}

	res, err := c.priorityService.FindPriority(reqCtx, id)
	if err != nil {
		return utils.ErrorResponse(ctx, err)
	}
	return utils.SuccessResponse(ctx, res, "Приоритет успешно найден", http.StatusOK)
}

func (c *PriorityController) CreatePriority(ctx echo.Context) error {
	reqCtx := ctx.Request().Context()
	c.logger.Debug("CreatePriority: Начало обработки запроса")

	dataString := ctx.FormValue("data")
	if dataString == "" {
		c.logger.Warn("CreatePriority: Поле 'data' отсутствует в form-data")
		return utils.ErrorResponse(ctx, apperrors.NewHttpError(http.StatusBadRequest, "Поле 'data' с JSON обязательно", nil))
	}
	c.logger.Debug("CreatePriority: Поле 'data' получено", zap.String("data", dataString))

	var dto dto.CreatePriorityDTO
	if err := json.Unmarshal([]byte(dataString), &dto); err != nil {
		c.logger.Error("CreatePriority: Ошибка парсинга JSON", zap.Error(err))
		return utils.ErrorResponse(ctx, apperrors.NewHttpError(http.StatusBadRequest, "Неверный JSON в 'data'", err))
	}

	if err := ctx.Validate(&dto); err != nil {
		c.logger.Error("CreatePriority: Ошибка валидации DTO", zap.Error(err))
		return utils.ErrorResponse(ctx, err)
	}
	c.logger.Debug("CreatePriority: DTO прошел валидацию")

	iconSmall, errSmall := ctx.FormFile("icon_small")
	if errSmall != nil && errSmall != http.ErrMissingFile {
		c.logger.Error("CreatePriority: Критическая ошибка при получении icon_small", zap.Error(errSmall))
		return utils.ErrorResponse(ctx, errSmall)
	}

	iconBig, errBig := ctx.FormFile("icon_big")
	if errBig != nil && errBig != http.ErrMissingFile {
		c.logger.Error("CreatePriority: Критическая ошибка при получении icon_big", zap.Error(errBig))
		return utils.ErrorResponse(ctx, errBig)
	}

	c.logger.Debug("CreatePriority: Вызов priorityService.CreatePriority")
	createdPriority, err := c.priorityService.CreatePriority(reqCtx, dto, iconSmall, iconBig)
	if err != nil {
		c.logger.Error("CreatePriority: Ошибка из сервиса", zap.Error(err))
		return utils.ErrorResponse(ctx, err)
	}

	c.logger.Info("CreatePriority: Приоритет успешно создан", zap.Any("result", createdPriority))
	return utils.SuccessResponse(ctx, createdPriority, "Приоритет успешно создан", http.StatusCreated)
}

func (c *PriorityController) UpdatePriority(ctx echo.Context) error {
	reqCtx := ctx.Request().Context()
	id, err := strconv.ParseUint(ctx.Param("id"), 10, 64)
	if err != nil {
		return utils.ErrorResponse(ctx, apperrors.NewHttpError(http.StatusBadRequest, "Неверный ID", err))
	}

	dataString := ctx.FormValue("data")
	var dto dto.UpdatePriorityDTO
	if dataString != "" {
		if err := json.Unmarshal([]byte(dataString), &dto); err != nil {
			return utils.ErrorResponse(ctx, apperrors.NewHttpError(http.StatusBadRequest, "Неверный JSON в 'data'", err))
		}
	}

	if err := ctx.Validate(&dto); err != nil {
		return utils.ErrorResponse(ctx, err)
	}

	iconSmall, errSmall := ctx.FormFile("icon_small")
	if errSmall != nil && errSmall != http.ErrMissingFile {
		return utils.ErrorResponse(ctx, errSmall)
	}
	iconBig, errBig := ctx.FormFile("icon_big")
	if errBig != nil && errBig != http.ErrMissingFile {
		return utils.ErrorResponse(ctx, errBig)
	}

	updatedPriority, err := c.priorityService.UpdatePriority(reqCtx, id, dto, iconSmall, iconBig)
	if err != nil {
		c.logger.Error("Ошибка при обновлении приоритета", zap.Uint64("id", id), zap.Error(err))
		return utils.ErrorResponse(ctx, err)
	}
	return utils.SuccessResponse(ctx, updatedPriority, "Приоритет успешно обновлен", http.StatusOK)
}

func (c *PriorityController) DeletePriority(ctx echo.Context) error {
	reqCtx := ctx.Request().Context()
	id, err := strconv.ParseUint(ctx.Param("id"), 10, 64)
	if err != nil {
		return utils.ErrorResponse(ctx, apperrors.NewHttpError(http.StatusBadRequest, "Неверный ID", err))
	}

	err = c.priorityService.DeletePriority(reqCtx, id)
	if err != nil {
		return utils.ErrorResponse(ctx, err)
	}
	return utils.SuccessResponse(ctx, struct{}{}, "Приоритет успешно удален", http.StatusOK)
}
