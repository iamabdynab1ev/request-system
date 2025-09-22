// Файл: internal/controllers/priority_controller.go
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
		c.logger.Error("GetPriorities: ошибка при получении списка приоритетов", zap.Error(err))
		return utils.ErrorResponse(ctx, apperrors.NewHttpError(
			http.StatusInternalServerError, "Не удалось получить список приоритетов", err, nil,
		), c.logger)
	}

	return utils.SuccessResponse(ctx, res.List, "Список приоритетов успешно получен", http.StatusOK, res.Pagination.TotalCount)
}

func (c *PriorityController) FindPriority(ctx echo.Context) error {
	reqCtx := ctx.Request().Context()
	idStr := ctx.Param("id")

	id, err := strconv.ParseUint(idStr, 10, 64)
	if err != nil {
		c.logger.Warn("FindPriority: неверный формат ID", zap.String("id", idStr), zap.Error(err))
		return utils.ErrorResponse(ctx, apperrors.NewHttpError(
			http.StatusBadRequest, "Неверный ID приоритета", err, map[string]interface{}{"param": idStr},
		), c.logger)
	}

	res, err := c.priorityService.FindPriority(reqCtx, id)
	if err != nil {
		c.logger.Error("FindPriority: ошибка при поиске приоритета", zap.Uint64("id", id), zap.Error(err))
		return utils.ErrorResponse(ctx, apperrors.NewHttpError(
			http.StatusInternalServerError, "Не удалось получить приоритет", err, nil,
		), c.logger)
	}

	return utils.SuccessResponse(ctx, res, "Приоритет успешно найден", http.StatusOK)
}

func (c *PriorityController) CreatePriority(ctx echo.Context) error {
	reqCtx := ctx.Request().Context()
	c.logger.Debug("CreatePriority: начало обработки JSON запроса")

	var dto dto.CreatePriorityDTO
	if err := ctx.Bind(&dto); err != nil {
		c.logger.Warn("CreatePriority: ошибка парсинга JSON тела запроса", zap.Error(err))
		return utils.ErrorResponse(ctx, apperrors.NewHttpError(
			http.StatusBadRequest, "Неверный формат JSON в теле запроса", err, nil,
		), c.logger)
	}

	if err := ctx.Validate(&dto); err != nil {
		c.logger.Warn("CreatePriority: ошибка валидации DTO", zap.Error(err))
		return utils.ErrorResponse(ctx, err, c.logger)
	}

	createdPriority, err := c.priorityService.CreatePriority(reqCtx, dto)
	if err != nil {
		c.logger.Error("CreatePriority: сервис вернул ошибку", zap.Error(err))
		return utils.ErrorResponse(ctx, err, c.logger)
	}

	c.logger.Info("CreatePriority: приоритет успешно создан", zap.Any("result", createdPriority))
	return utils.SuccessResponse(ctx, createdPriority, "Приоритет успешно создан", http.StatusCreated)
}

func (c *PriorityController) UpdatePriority(ctx echo.Context) error {
	reqCtx := ctx.Request().Context()
	id, err := strconv.ParseUint(ctx.Param("id"), 10, 64)
	if err != nil {
		c.logger.Warn("UpdatePriority: неверный ID", zap.String("id", ctx.Param("id")), zap.Error(err))
		return utils.ErrorResponse(ctx, apperrors.NewHttpError(
			http.StatusBadRequest, "Неверный формат ID приоритета", err, nil,
		), c.logger)
	}

	var dto dto.UpdatePriorityDTO
	if err := ctx.Bind(&dto); err != nil {
		c.logger.Warn("UpdatePriority: ошибка парсинга JSON тела запроса", zap.Error(err))
		return utils.ErrorResponse(ctx, apperrors.NewHttpError(
			http.StatusBadRequest, "Неверный формат JSON в теле запроса", err, nil,
		), c.logger)
	}

	if err := ctx.Validate(&dto); err != nil {
		c.logger.Warn("UpdatePriority: ошибка валидации DTO", zap.Error(err))
		return utils.ErrorResponse(ctx, err, c.logger)
	}

	updatedPriority, err := c.priorityService.UpdatePriority(reqCtx, id, dto)
	if err != nil {
		c.logger.Error("UpdatePriority: сервис вернул ошибку", zap.Uint64("id", id), zap.Error(err))
		return utils.ErrorResponse(ctx, err, c.logger)
	}

	return utils.SuccessResponse(ctx, updatedPriority, "Приоритет успешно обновлен", http.StatusOK)
}

func (c *PriorityController) DeletePriority(ctx echo.Context) error {
	reqCtx := ctx.Request().Context()
	id, err := strconv.ParseUint(ctx.Param("id"), 10, 64)
	if err != nil {
		c.logger.Warn("DeletePriority: неверный ID", zap.String("id", ctx.Param("id")), zap.Error(err))
		return utils.ErrorResponse(ctx, apperrors.NewHttpError(
			http.StatusBadRequest, "Неверный формат ID приоритета", err, nil,
		), c.logger)
	}

	if err := c.priorityService.DeletePriority(reqCtx, id); err != nil {
		c.logger.Error("DeletePriority: ошибка при удалении приоритета", zap.Uint64("id", id), zap.Error(err))
		return utils.ErrorResponse(ctx, apperrors.NewHttpError(
			http.StatusInternalServerError, "Не удалось удалить приоритет", err, nil,
		), c.logger)
	}

	return utils.SuccessResponse(ctx, struct{}{}, "Приоритет успешно удален", http.StatusOK)
}
