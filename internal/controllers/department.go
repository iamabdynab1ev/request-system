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

func NewDepartmentController(
	departmentService *services.DepartmentService,
	logger *zap.Logger,
) *DepartmentController {
	return &DepartmentController{
		departmentService: departmentService,
		logger:            logger,
	}
}

// GetDepartments теперь поддерживает пагинацию, фильтры и поиск
func (c *DepartmentController) GetDepartments(ctx echo.Context) error {
	reqCtx := ctx.Request().Context()

	// Парсим параметры запроса (limit, page, sort, filter, search)
	filter := utils.ParseFilterFromQuery(ctx.Request().URL.Query())

	// Вызываем сервис с параметрами
	departments, total, err := c.departmentService.GetDepartments(reqCtx, filter)
	if err != nil {
		return utils.ErrorResponse(ctx, err)
	}

	// Возвращаем успешный ответ с пагинацией
	return utils.SuccessResponse(ctx, departments, "Successfully fetched departments", http.StatusOK, total)
}

func (c *DepartmentController) FindDepartment(ctx echo.Context) error {
	reqCtx := ctx.Request().Context()

	id, err := strconv.ParseUint(ctx.Param("id"), 10, 64)
	if err != nil {
		return utils.ErrorResponse(ctx, echo.NewHTTPError(http.StatusBadRequest, "Invalid ID format"))
	}

	res, err := c.departmentService.FindDepartment(reqCtx, id)
	if err != nil {
		return utils.ErrorResponse(ctx, err)
	}

	return utils.SuccessResponse(ctx, res, "Successfully found department", http.StatusOK)
}

func (c *DepartmentController) CreateDepartment(ctx echo.Context) error {
	reqCtx := ctx.Request().Context()

	var dto dto.CreateDepartmentDTO
	if err := ctx.Bind(&dto); err != nil {
		c.logger.Error("неправильный запрос", zap.Error(err))
		return utils.ErrorResponse(ctx, echo.NewHTTPError(http.StatusBadRequest, "Invalid request body"))
	}

	if err := ctx.Validate(&dto); err != nil {
		c.logger.Error("Ошибка при валидации данных департамента", zap.Error(err))
		return utils.ErrorResponse(ctx, err) // Валидатор echo уже вернет HTTPError
	}

	// Сервис теперь возвращает созданный объект, который мы передаем в ответ
	res, err := c.departmentService.CreateDepartment(reqCtx, dto)
	if err != nil {
		c.logger.Error("Ошибка при создании департамента", zap.Error(err))
		return utils.ErrorResponse(ctx, err)
	}

	return utils.SuccessResponse(ctx, res, "Successfully created department", http.StatusCreated)
}

func (c *DepartmentController) UpdateDepartment(ctx echo.Context) error {
	reqCtx := ctx.Request().Context()

	id, err := strconv.ParseUint(ctx.Param("id"), 10, 64)
	if err != nil {
		return utils.ErrorResponse(ctx, echo.NewHTTPError(http.StatusBadRequest, "Invalid ID format"))
	}

	var dto dto.UpdateDepartmentDTO
	if err := ctx.Bind(&dto); err != nil {
		return utils.ErrorResponse(ctx, echo.NewHTTPError(http.StatusBadRequest, "Invalid request body"))
	}

	if err := ctx.Validate(&dto); err != nil {
		c.logger.Error("Ошибка при валидации данных департамента", zap.Error(err))
		return utils.ErrorResponse(ctx, err)
	}

	// Сервис теперь возвращает обновленный объект
	res, err := c.departmentService.UpdateDepartment(reqCtx, id, dto)
	if err != nil {
		return utils.ErrorResponse(ctx, err)
	}

	return utils.SuccessResponse(ctx, res, "Successfully updated department", http.StatusOK)
}

func (c *DepartmentController) DeleteDepartment(ctx echo.Context) error {
	reqCtx := ctx.Request().Context()

	id, err := strconv.ParseUint(ctx.Param("id"), 10, 64)
	if err != nil {
		return utils.ErrorResponse(ctx, echo.NewHTTPError(http.StatusBadRequest, "Invalid ID format"))
	}

	err = c.departmentService.DeleteDepartment(reqCtx, id)
	if err != nil {
		return utils.ErrorResponse(ctx, err)
	}

	// Используем nil вместо struct{}{}, SuccessResponse обработает это корректно
	return utils.SuccessResponse(ctx, nil, "Successfully deleted department", http.StatusOK)
}
