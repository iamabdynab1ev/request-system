package controllers

import (
	"fmt"
	"net/http"
	"strconv"

	"request-system/internal/dto"
	"request-system/internal/services" // Используем RoleServiceInterface
	apperrors "request-system/pkg/errors"
	"request-system/pkg/utils" // Используем ErrorResponse, SuccessResponse

	"go.uber.org/zap"

	"github.com/labstack/echo/v4"
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
	filter := utils.ParseFilterFromQuery(ctx.Request().URL.Query())

	paginatedResponse, err := c.roleService.GetRoles(reqCtx, uint64(filter.Limit), uint64(filter.Offset))
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
	id, err := strconv.ParseUint(ctx.Param("id"), 10, 64) // id теперь uint64
	if err != nil {
		c.logger.Error("некорректный ID роли", zap.String("id", ctx.Param("id")), zap.Error(err))
		return utils.ErrorResponse(ctx, fmt.Errorf("некорректный ID роли: %w", apperrors.ErrBadRequest)) // Единообразные ошибки
	}
	res, err := c.roleService.FindRole(ctx.Request().Context(), id)
	if err != nil {
		c.logger.Error("ошибка при поиске роли по ID", zap.Uint64("id", id), zap.Error(err)) // zap.Uint64 для логирования
		return utils.ErrorResponse(ctx, err)
	}
	return utils.SuccessResponse(ctx, res, "Роль успешно найдена", http.StatusOK)
}

func (c *RoleController) CreateRole(ctx echo.Context) error {
	var dto dto.CreateRoleDTO
	if err := ctx.Bind(&dto); err != nil {
		c.logger.Error("CreateRole: неверный запрос (bind)", zap.Error(err))
		return utils.ErrorResponse(ctx, fmt.Errorf("неверный формат запроса: %w", apperrors.ErrBadRequest))
	}
	if err := ctx.Validate(&dto); err != nil {
		c.logger.Error("CreateRole: ошибка валидации данных", zap.Error(err))
		return utils.ErrorResponse(ctx, err)
	}
	res, err := c.roleService.CreateRole(ctx.Request().Context(), dto)
	if err != nil {
		c.logger.Error("CreateRole: ошибка при создании роли", zap.Error(err))
		return utils.ErrorResponse(ctx, err)
	}
	return utils.SuccessResponse(ctx, res, "Роль успешно создана", http.StatusCreated)
}

func (c *RoleController) UpdateRole(ctx echo.Context) error {
	id, err := strconv.ParseUint(ctx.Param("id"), 10, 64) // id теперь uint64
	if err != nil {
		c.logger.Error("UpdateRole: некорректный ID роли для обновления", zap.String("id", ctx.Param("id")), zap.Error(err))
		return utils.ErrorResponse(ctx, fmt.Errorf("некорректный ID роли: %w", apperrors.ErrBadRequest))
	}
	var dto dto.UpdateRoleDTO
	if err := ctx.Bind(&dto); err != nil {
		c.logger.Error("UpdateRole: неверный запрос (bind)", zap.Error(err))
		return utils.ErrorResponse(ctx, fmt.Errorf("неверный формат запроса: %w", apperrors.ErrBadRequest))
	}
	if err := ctx.Validate(&dto); err != nil {
		c.logger.Error("UpdateRole: ошибка валидации данных", zap.Error(err))
		return utils.ErrorResponse(ctx, err)
	}
	res, err := c.roleService.UpdateRole(ctx.Request().Context(), id, dto)
	if err != nil {
		c.logger.Error("UpdateRole: ошибка при обновлении роли", zap.Uint64("id", id), zap.Error(err))
		return utils.ErrorResponse(ctx, err)
	}
	return utils.SuccessResponse(ctx, res, "Роль успешно обновлена", http.StatusOK)
}

func (c *RoleController) DeleteRole(ctx echo.Context) error {
	id, err := strconv.ParseUint(ctx.Param("id"), 10, 64) // id теперь uint64
	if err != nil {
		c.logger.Error("DeleteRole: некорректный ID роли", zap.String("id", ctx.Param("id")), zap.Error(err))
		return utils.ErrorResponse(ctx, fmt.Errorf("некорректный ID роли: %w", apperrors.ErrBadRequest))
	}
	err = c.roleService.DeleteRole(ctx.Request().Context(), id)
	if err != nil {
		c.logger.Error("DeleteRole: ошибка при удалении роли", zap.Uint64("id", id), zap.Error(err))
		return utils.ErrorResponse(ctx, err)
	}
	return utils.SuccessResponse(ctx, struct{}{}, "Роль успешно удалена", http.StatusOK)
}
