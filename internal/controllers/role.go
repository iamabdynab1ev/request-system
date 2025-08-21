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

type RoleController struct {
	roleService services.RoleServiceInterface
	logger      *zap.Logger
}

func NewRoleController(roleService services.RoleServiceInterface, logger *zap.Logger) *RoleController {
	return &RoleController{roleService: roleService, logger: logger}
}

func (c *RoleController) GetRoles(ctx echo.Context) error {
	filter := utils.ParseFilterFromQuery(ctx.Request().URL.Query())
	res, total, err := c.roleService.GetRoles(ctx.Request().Context(), filter)
	if err != nil {
		c.logger.Error("ошибка получения списка ролей", zap.Error(err))
		return utils.ErrorResponse(ctx, err)
	}
	return utils.SuccessResponse(ctx, res, "Список ролей успешно получен", http.StatusOK, total)
}

func (c *RoleController) FindRole(ctx echo.Context) error {
	id, err := strconv.ParseUint(ctx.Param("id"), 10, 64)
	if err != nil {
		return utils.ErrorResponse(ctx, apperrors.NewBadRequestError("Неверный формат ID роли"))
	}
	res, err := c.roleService.FindRole(ctx.Request().Context(), id)
	if err != nil {
		c.logger.Error("ошибка поиска роли", zap.Uint64("id", id), zap.Error(err))
		return utils.ErrorResponse(ctx, err)
	}
	return utils.SuccessResponse(ctx, res, "Роль успешно найдена", http.StatusOK)
}

func (c *RoleController) CreateRole(ctx echo.Context) error {
	var dto dto.CreateRoleDTO
	if err := ctx.Bind(&dto); err != nil {
		return utils.ErrorResponse(ctx, apperrors.NewBadRequestError("Неверный формат запроса"))
	}
	if err := ctx.Validate(&dto); err != nil {
		return utils.ErrorResponse(ctx, err)
	}

	res, err := c.roleService.CreateRole(ctx.Request().Context(), dto)
	if err != nil {
		c.logger.Error("ошибка создания роли", zap.Error(err))
		return utils.ErrorResponse(ctx, err)
	}
	return utils.SuccessResponse(ctx, res, "Роль успешно создана", http.StatusCreated)
}

func (c *RoleController) UpdateRole(ctx echo.Context) error {
	id, err := strconv.ParseUint(ctx.Param("id"), 10, 64)
	if err != nil {
		return utils.ErrorResponse(ctx, apperrors.NewBadRequestError("Неверный формат ID роли"))
	}

	var dto dto.UpdateRoleDTO
	if err := ctx.Bind(&dto); err != nil {
		return utils.ErrorResponse(ctx, apperrors.NewBadRequestError("Неверный формат запроса"))
	}
	if err := ctx.Validate(&dto); err != nil {
		return utils.ErrorResponse(ctx, err)
	}

	res, err := c.roleService.UpdateRole(ctx.Request().Context(), id, dto)
	if err != nil {
		c.logger.Error("ошибка обновления роли", zap.Uint64("id", id), zap.Error(err))
		return utils.ErrorResponse(ctx, err)
	}
	return utils.SuccessResponse(ctx, res, "Роль успешно обновлена", http.StatusOK)
}

func (c *RoleController) DeleteRole(ctx echo.Context) error {
	id, err := strconv.ParseUint(ctx.Param("id"), 10, 64)
	if err != nil {
		return utils.ErrorResponse(ctx, apperrors.NewBadRequestError("Неверный формат ID роли"))
	}

	if err := c.roleService.DeleteRole(ctx.Request().Context(), id); err != nil {
		c.logger.Error("ошибка удаления роли", zap.Uint64("id", id), zap.Error(err))
		return utils.ErrorResponse(ctx, err)
	}
	return utils.SuccessResponse(ctx, struct{}{}, "Роль успешно удалена", http.StatusOK)
}
