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
	reqCtx := ctx.Request().Context()
	filter := utils.ParseFilterFromQuery(ctx.Request().URL.Query())

	res, total, err := c.roleService.GetRoles(reqCtx, filter)
	if err != nil {
		c.logger.Error("GetRoles: ошибка получения списка ролей", zap.Error(err))
		return utils.ErrorResponse(ctx, err, c.logger)
	}

	return utils.SuccessResponse(ctx, res, "Список ролей успешно получен", http.StatusOK, total)
}

func (c *RoleController) FindRole(ctx echo.Context) error {
	reqCtx := ctx.Request().Context()
	idParam := ctx.Param("id")

	id, err := strconv.ParseUint(idParam, 10, 64)
	if err != nil {
		c.logger.Warn("FindRole: Неверный формат ID роли", zap.String("param", idParam), zap.Error(err))
		return utils.ErrorResponse(ctx,
			apperrors.NewHttpError(
				http.StatusBadRequest,
				"Неверный формат ID роли",
				err,
				map[string]interface{}{"param": idParam},
			),
			c.logger,
		)
	}

	res, err := c.roleService.FindRole(reqCtx, id)
	if err != nil {
		c.logger.Error("FindRole: ошибка поиска роли", zap.Uint64("id", id), zap.Error(err))
		return utils.ErrorResponse(ctx, err, c.logger)
	}

	return utils.SuccessResponse(ctx, res, "Роль успешно найдена", http.StatusOK)
}

func (c *RoleController) CreateRole(ctx echo.Context) error {
	reqCtx := ctx.Request().Context()

	var dto dto.CreateRoleDTO
	if err := ctx.Bind(&dto); err != nil {
		c.logger.Warn("CreateRole: Неверный формат запроса", zap.Error(err))
		return utils.ErrorResponse(ctx,
			apperrors.NewHttpError(
				http.StatusBadRequest,
				"Неверный формат запроса",
				err,
				nil,
			),
			c.logger,
		)
	}

	if err := ctx.Validate(&dto); err != nil {
		c.logger.Error("CreateRole: Ошибка валидации DTO", zap.Error(err))
		return utils.ErrorResponse(ctx, err, c.logger)
	}

	res, err := c.roleService.CreateRole(reqCtx, dto)
	if err != nil {
		c.logger.Error("CreateRole: Ошибка создания роли", zap.Error(err))
		return utils.ErrorResponse(ctx, err, c.logger)
	}

	return utils.SuccessResponse(ctx, res, "Роль успешно создана", http.StatusCreated)
}

func (c *RoleController) UpdateRole(ctx echo.Context) error {
	reqCtx := ctx.Request().Context()

	id, err := strconv.ParseUint(ctx.Param("id"), 10, 64)
	if err != nil {
		c.logger.Warn("UpdateRole: Неверный формат ID роли", zap.String("param", ctx.Param("id")), zap.Error(err))
		return utils.ErrorResponse(ctx,
			apperrors.NewHttpError(
				http.StatusBadRequest,
				"Неверный формат ID роли",
				err,
				map[string]interface{}{"param": ctx.Param("id")},
			),
			c.logger,
		)
	}

	var dto dto.UpdateRoleDTO
	if err := ctx.Bind(&dto); err != nil {
		c.logger.Warn("UpdateRole: Неверный формат запроса", zap.Error(err))
		return utils.ErrorResponse(ctx,
			apperrors.NewHttpError(
				http.StatusBadRequest,
				"Неверный формат запроса",
				err,
				nil,
			),
			c.logger,
		)
	}

	if err := ctx.Validate(&dto); err != nil {
		c.logger.Error("UpdateRole: Ошибка валидации DTO", zap.Error(err))
		return utils.ErrorResponse(ctx, err, c.logger)
	}

	res, err := c.roleService.UpdateRole(reqCtx, id, dto)
	if err != nil {
		c.logger.Error("UpdateRole: Ошибка при обновлении роли", zap.Uint64("id", id), zap.Error(err))
		return utils.ErrorResponse(ctx, err, c.logger)
	}

	return utils.SuccessResponse(ctx, res, "Роль успешно обновлена", http.StatusOK)
}

func (c *RoleController) DeleteRole(ctx echo.Context) error {
	reqCtx := ctx.Request().Context()

	id, err := strconv.ParseUint(ctx.Param("id"), 10, 64)
	if err != nil {
		c.logger.Warn("DeleteRole: Неверный формат ID роли", zap.String("param", ctx.Param("id")), zap.Error(err))
		return utils.ErrorResponse(ctx,
			apperrors.NewHttpError(
				http.StatusBadRequest,
				"Неверный формат ID роли",
				err,
				map[string]interface{}{"param": ctx.Param("id")},
			),
			c.logger,
		)
	}

	if err := c.roleService.DeleteRole(reqCtx, id); err != nil {
		c.logger.Error("DeleteRole: Ошибка при удалении роли", zap.Uint64("id", id), zap.Error(err))
		return utils.ErrorResponse(ctx, err, c.logger)
	}

	return utils.SuccessResponse(ctx, struct{}{}, "Роль успешно удалена", http.StatusOK)
}
