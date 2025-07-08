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

type RoleController struct {
	roleService services.RoleServiceInterface
	logger      *zap.Logger
}

func NewRoleController(roleService services.RoleServiceInterface, logger *zap.Logger) *RoleController {
	return &RoleController{
		roleService: roleService,
		logger:      logger,
	}
}

func (c *RoleController) GetRoles(ctx echo.Context) error {
	reqCtx := ctx.Request().Context()
	limit, offset, _ := utils.ParsePaginationParams(ctx.QueryParams())
	paginatedResponse, err := c.roleService.GetRoles(reqCtx, limit, offset)
	if err != nil {
		c.logger.Error("ошибка в контроллере при получении списка ролей", zap.Error(err))
		return utils.ErrorResponse(ctx, err)
	}

	return utils.SuccessResponse(
		ctx,
		paginatedResponse,
		"Список ролей успешно получен",
		http.StatusOK,
	)
}
func (c *RoleController) FindRole(ctx echo.Context) error {
	id, err := strconv.ParseUint(ctx.Param("id"), 10, 64)
	if err != nil {
		c.logger.Error("некорректный ID роли", zap.String("id", ctx.Param("id")), zap.Error(err))
		return utils.ErrorResponse(ctx, echo.NewHTTPError(http.StatusBadRequest, "Некорректный ID роли"))
	}
	res, err := c.roleService.FindRole(ctx.Request().Context(), id)
	if err != nil {
		return utils.ErrorResponse(ctx, err)
	}
	return utils.SuccessResponse(ctx, res, "Роль успешно найдена", http.StatusOK)
}

func (c *RoleController) CreateRole(ctx echo.Context) error {
	var dto dto.CreateRoleDTO
	if err := ctx.Bind(&dto); err != nil {
		c.logger.Error("неверный запрос на создание роли (bind)", zap.Error(err))
		return utils.ErrorResponse(ctx, err)
	}
	if err := ctx.Validate(&dto); err != nil {
		c.logger.Error("ошибка валидации данных для новой роли", zap.Error(err))
		return utils.ErrorResponse(ctx, err)
	}
	res, err := c.roleService.CreateRole(ctx.Request().Context(), dto)
	if err != nil {
		c.logger.Error("ошибка при создании роли в контроллере", zap.Error(err))
		return utils.ErrorResponse(ctx, err)
	}
	return utils.SuccessResponse(ctx, res, "Роль успешно создана", http.StatusCreated)
}

func (c *RoleController) UpdateRole(ctx echo.Context) error {
	id, err := strconv.ParseUint(ctx.Param("id"), 10, 64)
	if err != nil {
		c.logger.Error("некорректный ID роли для обновления", zap.String("id", ctx.Param("id")), zap.Error(err))
		return utils.ErrorResponse(ctx, echo.NewHTTPError(http.StatusBadRequest, "Некорректный ID роли"))
	}
	var dto dto.UpdateRoleDTO
	if err := ctx.Bind(&dto); err != nil {
		return utils.ErrorResponse(ctx, err)
	}
	if err := ctx.Validate(&dto); err != nil {
		return utils.ErrorResponse(ctx, err)
	}
	res, err := c.roleService.UpdateRole(ctx.Request().Context(), id, dto)
	if err != nil {
		return utils.ErrorResponse(ctx, err)
	}
	return utils.SuccessResponse(ctx, res, "Роль успешно обновлена", http.StatusOK)
}

func (c *RoleController) DeleteRole(ctx echo.Context) error {
	id, err := strconv.ParseUint(ctx.Param("id"), 10, 64)
	if err != nil {
		return utils.ErrorResponse(ctx, echo.NewHTTPError(http.StatusBadRequest, "Некорректный ID роли"))
	}
	err = c.roleService.DeleteRole(ctx.Request().Context(), id)
	if err != nil {
		return utils.ErrorResponse(ctx, err)
	}
	return utils.SuccessResponse(ctx, struct{}{}, "Роль успешно удалена", http.StatusOK)
}
