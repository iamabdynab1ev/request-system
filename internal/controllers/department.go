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

type DepartmentController struct {
	departmentService services.DepartmentServiceInterface
	logger            *zap.Logger
}

func NewDepartmentController(service services.DepartmentServiceInterface, logger *zap.Logger) *DepartmentController {
	return &DepartmentController{departmentService: service, logger: logger}
}

func (c *DepartmentController) GetDepartments(ctx echo.Context) error {
	filter := utils.ParseFilterFromQuery(ctx.Request().URL.Query())

	departments, total, err := c.departmentService.GetDepartments(ctx.Request().Context(), filter)
	if err != nil {
		c.logger.Error("Ошибка при получении списка департаментов", zap.Error(err))
		return utils.ErrorResponse(ctx, err, c.logger)
	}

	return utils.SuccessResponse(ctx, departments, "Список департаментов успешно получен", http.StatusOK, total)
}

func (c *DepartmentController) FindDepartment(ctx echo.Context) error {
	id, err := strconv.ParseUint(ctx.Param("id"), 10, 64)
	if err != nil {
		c.logger.Error("FindDepartment: неверный формат ID", zap.String("id", ctx.Param("id")), zap.Error(err))
		return utils.ErrorResponse(
			ctx,
			apperrors.NewHttpError(
				http.StatusBadRequest,
				"Неверный формат ID департамента",
				err,
				map[string]interface{}{"param": ctx.Param("id")},
			),
			c.logger,
		)
	}

	dept, err := c.departmentService.FindDepartment(ctx.Request().Context(), id)
	if err != nil {
		c.logger.Error("Ошибка при поиске департамента", zap.Uint64("id", id), zap.Error(err))
		return utils.ErrorResponse(
			ctx,
			apperrors.NewHttpError(
				http.StatusInternalServerError,
				"Не удалось найти департамент",
				err,
				nil,
			),
			c.logger,
		)
	}

	return utils.SuccessResponse(ctx, dept, "Департамент успешно найден", http.StatusOK)
}

func (c *DepartmentController) CreateDepartment(ctx echo.Context) error {
	var dto dto.CreateDepartmentDTO
	if err := ctx.Bind(&dto); err != nil {
		c.logger.Error("CreateDepartment: ошибка привязки данных", zap.Error(err))
		return utils.ErrorResponse(
			ctx,
			apperrors.NewHttpError(
				http.StatusBadRequest,
				"Неверный формат данных в теле запроса",
				err,
				nil,
			),
			c.logger,
		)
	}
	if err := ctx.Validate(&dto); err != nil {
		c.logger.Error("CreateDepartment: ошибка валидации данных", zap.Error(err))
		return utils.ErrorResponse(ctx, err, c.logger)
	}
	res, err := c.departmentService.CreateDepartment(ctx.Request().Context(), dto)
	if err != nil {
		c.logger.Error("Ошибка при создании департамента", zap.Any("payload", dto), zap.Error(err))
		return utils.ErrorResponse(ctx, err, c.logger)
	}
	return utils.SuccessResponse(ctx, res, "Департамент успешно создан", http.StatusCreated)
}

func (c *DepartmentController) UpdateDepartment(ctx echo.Context) error {
	id, err := strconv.ParseUint(ctx.Param("id"), 10, 64)
	if err != nil {
		c.logger.Error("UpdateDepartment: неверный формат ID", zap.String("id", ctx.Param("id")), zap.Error(err))
		return utils.ErrorResponse(
			ctx,
			apperrors.NewHttpError(http.StatusBadRequest, "Неверный формат ID департамента", err, nil),
			c.logger,
		)
	}

	// ---- НОВАЯ, ПРОСТАЯ И НАДЕЖНАЯ ЛОГИКА ПАРСИНГА ----
	var dto dto.UpdateDepartmentDTO

	// Мы просто читаем тело запроса и пытаемся его распарсить как JSON.
	if err := json.NewDecoder(ctx.Request().Body).Decode(&dto); err != nil {
		c.logger.Error("UpdateDepartment: ошибка привязки данных из JSON-тела", zap.Error(err))
		return utils.ErrorResponse(
			ctx,
			apperrors.NewHttpError(http.StatusBadRequest, "Неверный формат данных в теле запроса", err, nil),
			c.logger,
		)
	}

	// После парсинга логгируем, чтобы УБЕДИТЬСЯ, что данные прочитались.
	c.logger.Debug("DTO после парсинга в контроллере", zap.Any("parsedDTO", dto))
	// --------------------------------------------------------

	if err := ctx.Validate(&dto); err != nil {
		c.logger.Error("UpdateDepartment: ошибка валидации данных", zap.Error(err))
		return utils.ErrorResponse(ctx, err, c.logger)
	}

	res, err := c.departmentService.UpdateDepartment(ctx.Request().Context(), id, dto)
	if err != nil {
		c.logger.Error("Ошибка при обновлении департамента", zap.Uint64("id", id), zap.Any("payload", dto), zap.Error(err))
		return utils.ErrorResponse(
			ctx,
			err, // Передаем ошибку из сервиса/репозитория напрямую
			c.logger,
		)
	}

	return utils.SuccessResponse(ctx, res, "Департамент успешно обновлен", http.StatusOK)
}

func (c *DepartmentController) DeleteDepartment(ctx echo.Context) error {
	id, err := strconv.ParseUint(ctx.Param("id"), 10, 64)
	if err != nil {
		c.logger.Error("DeleteDepartment: неверный формат ID", zap.String("id", ctx.Param("id")), zap.Error(err))
		return utils.ErrorResponse(
			ctx,
			apperrors.NewHttpError(
				http.StatusBadRequest,
				"Неверный формат ID департамента",
				err,
				map[string]interface{}{"param": ctx.Param("id")},
			),
			c.logger,
		)
	}
	if err := c.departmentService.DeleteDepartment(ctx.Request().Context(), id); err != nil {
		c.logger.Error("Ошибка при удалении департамента", zap.Uint64("id", id), zap.Error(err))
		return utils.ErrorResponse(ctx, err, c.logger)
	}
	return utils.SuccessResponse(ctx, nil, "Департамент успешно удален", http.StatusOK)
}

func (c *DepartmentController) GetDepartmentStats(ctx echo.Context) error {
	filter := utils.ParseFilterFromQuery(ctx.Request().URL.Query())
	stats, total, err := c.departmentService.GetDepartmentStats(ctx.Request().Context(), filter)
	if err != nil {
		c.logger.Error("Ошибка при получении статистики департаментов", zap.Error(err))
		return utils.ErrorResponse(ctx, err, c.logger)
	}
	return utils.SuccessResponse(ctx, stats, "Статистика по департаментам успешно получена", http.StatusOK, total)
}
