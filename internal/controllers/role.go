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

type RoleController struct {
	roleService services.RoleServiceInterface
	logger      *zap.Logger
}

func NewRoleController(roleService services.RoleServiceInterface, logger *zap.Logger) *RoleController {
	return &RoleController{roleService: roleService, logger: logger}
}

func (c *RoleController) GetRoles(ctx echo.Context) error {
	reqCtx := ctx.Request().Context()
	filter := utils.ParseFilterFromQuery(ctx.Request().URL.Query())

	paginatedResponse, err := c.roleService.GetRoles(reqCtx, uint64(filter.Limit), uint64(filter.Offset))
	if err != nil {
		c.logger.Error("ошибка в контроллере при получении списка ролей", zap.Error(err))
		return utils.ErrorResponse(ctx, err)
	}

	return utils.SuccessResponse(ctx, paginatedResponse.List, "Список ролей успешно получен", http.StatusOK, paginatedResponse.Pagination.TotalCount)
}

func (c *RoleController) FindRole(ctx echo.Context) error {
	id, err := strconv.ParseUint(ctx.Param("id"), 10, 64)
	if err != nil {
		return utils.ErrorResponse(ctx, apperrors.NewHttpError(http.StatusBadRequest, "Некорректный ID роли", err))
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
		return utils.ErrorResponse(ctx, apperrors.NewHttpError(http.StatusBadRequest, "Неверный формат запроса", err))
	}
	if err := ctx.Validate(&dto); err != nil {
		return utils.ErrorResponse(ctx, err)
	}
	res, err := c.roleService.CreateRole(ctx.Request().Context(), dto)
	if err != nil {
		return utils.ErrorResponse(ctx, err)
	}
	return utils.SuccessResponse(ctx, res, "Роль успешно создана", http.StatusCreated)
}

func (c *RoleController) UpdateRole(ctx echo.Context) error {
	id, err := strconv.ParseUint(ctx.Param("id"), 10, 64)
	if err != nil {
		return utils.ErrorResponse(ctx, apperrors.NewHttpError(http.StatusBadRequest, "Некорректный ID роли", err))
	}
	var dto dto.UpdateRoleDTO
	if err := ctx.Bind(&dto); err != nil {
		return utils.ErrorResponse(ctx, apperrors.NewHttpError(http.StatusBadRequest, "Неверный формат запроса", err))
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
		return utils.ErrorResponse(ctx, apperrors.NewHttpError(http.StatusBadRequest, "Некорректный ID роли", err))
	}
	if err := c.roleService.DeleteRole(ctx.Request().Context(), id); err != nil {
		return utils.ErrorResponse(ctx, err)
	}
	return utils.SuccessResponse(ctx, struct{}{}, "Роль успешно удалена", http.StatusOK)
}
