package controllers

import (
	"net/http"
	"request-system/internal/dto"
	"request-system/internal/services"
	"request-system/pkg/utils"
	"strconv"

	"github.com/labstack/echo/v4"
	"go.uber.org/zap"
)

type DepartmentController struct {
	departmentService *services.DepartmentService
	logger            *zap.Logger
}

func NewDepartmentController(service *services.DepartmentService, logger *zap.Logger) *DepartmentController {
	return &DepartmentController{departmentService: service, logger: logger}
}

// Новый контроллер для статистики
func (c *DepartmentController) GetDepartmentStats(ctx echo.Context) error {
	filter := utils.ParseFilterFromQuery(ctx.Request().URL.Query())
	stats, total, err := c.departmentService.GetDepartmentStats(ctx.Request().Context(), filter)
	if err != nil {
		return utils.ErrorResponse(ctx, err)
	}
	return utils.SuccessResponse(ctx, stats, "Статистика по департаментам успешно получена", http.StatusOK, total)
}

// Существующий контроллер, теперь вызывает обновленный сервис
func (c *DepartmentController) GetDepartments(ctx echo.Context) error {
	filter := utils.ParseFilterFromQuery(ctx.Request().URL.Query())
	departments, total, err := c.departmentService.GetDepartments(ctx.Request().Context(), filter)
	if err != nil {
		return utils.ErrorResponse(ctx, err)
	}
	return utils.SuccessResponse(ctx, departments, "Департаменты успешно получены", http.StatusOK, total)
}

// ----- Остальные методы контроллера -----

func (c *DepartmentController) FindDepartment(ctx echo.Context) error {
	id, err := strconv.ParseUint(ctx.Param("id"), 10, 64)
	if err != nil {
		return utils.ErrorResponse(ctx, echo.NewHTTPError(http.StatusBadRequest, "Неверный формат ID"))
	}
	res, err := c.departmentService.FindDepartment(ctx.Request().Context(), id)
	if err != nil {
		return utils.ErrorResponse(ctx, err)
	}
	return utils.SuccessResponse(ctx, res, "Департамент успешно найден", http.StatusOK)
}

func (c *DepartmentController) CreateDepartment(ctx echo.Context) error {
	var dto dto.CreateDepartmentDTO
	if err := ctx.Bind(&dto); err != nil {
		return utils.ErrorResponse(ctx, echo.NewHTTPError(http.StatusBadRequest, "Неверное тело запроса"))
	}
	if err := ctx.Validate(&dto); err != nil {
		return utils.ErrorResponse(ctx, err)
	}
	res, err := c.departmentService.CreateDepartment(ctx.Request().Context(), dto)
	if err != nil {
		return utils.ErrorResponse(ctx, err)
	}
	return utils.SuccessResponse(ctx, res, "Департамент успешно создан", http.StatusCreated)
}

func (c *DepartmentController) UpdateDepartment(ctx echo.Context) error {
	id, err := strconv.ParseUint(ctx.Param("id"), 10, 64)
	if err != nil {
		return utils.ErrorResponse(ctx, echo.NewHTTPError(http.StatusBadRequest, "Неверный формат ID"))
	}
	var dto dto.UpdateDepartmentDTO
	if err := ctx.Bind(&dto); err != nil {
		return utils.ErrorResponse(ctx, echo.NewHTTPError(http.StatusBadRequest, "Неверное тело запроса"))
	}
	if err := ctx.Validate(&dto); err != nil {
		return utils.ErrorResponse(ctx, err)
	}
	res, err := c.departmentService.UpdateDepartment(ctx.Request().Context(), id, dto)
	if err != nil {
		return utils.ErrorResponse(ctx, err)
	}
	return utils.SuccessResponse(ctx, res, "Департамент успешно обновлен", http.StatusOK)
}

func (c *DepartmentController) DeleteDepartment(ctx echo.Context) error {
	id, err := strconv.ParseUint(ctx.Param("id"), 10, 64)
	if err != nil {
		return utils.ErrorResponse(ctx, echo.NewHTTPError(http.StatusBadRequest, "Неверный формат ID"))
	}
	if err := c.departmentService.DeleteDepartment(ctx.Request().Context(), id); err != nil {
		return utils.ErrorResponse(ctx, err)
	}
	return utils.SuccessResponse(ctx, nil, "Департамент успешно удален", http.StatusOK)
}
