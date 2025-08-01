package controllers

import (
	"fmt"
	"net/http"
	"strconv"

	"request-system/internal/dto"
	"request-system/internal/services" // <-- ВЫ УЖЕ ЭТО ИСПОЛЬЗУЕТЕ

	apperrors "request-system/pkg/errors"
	"request-system/pkg/utils"

	"go.uber.org/zap"

	"github.com/labstack/echo/v4"
)

type RolePermissionController struct {
	// ИСПРАВЛЕНО ЗДЕСЬ: rpService теперь интерфейсный тип
	rpService services.RolePermissionServiceInterface // <-- ИЗМЕНЕНО: был *services.RolePermissionService
	logger    *zap.Logger
}

// ИСПРАВЛЕНО: Конструктор теперь принимает интерфейсный тип
func NewRolePermissionController(
	rpService services.RolePermissionServiceInterface, // <-- ИЗМЕНЕНО: был *services.RolePermissionService
	logger *zap.Logger,
) *RolePermissionController {
	return &RolePermissionController{
		rpService: rpService,
		logger:    logger,
	}
}

func (c *RolePermissionController) GetRolePermissions(ctx echo.Context) error {
	reqCtx := ctx.Request().Context()
	filter := utils.ParseFilterFromQuery(ctx.Request().URL.Query())

	rpList, rpTotal, err := c.rpService.GetRolePermissions(reqCtx, uint64(filter.Limit), uint64(filter.Offset))
	if err != nil {
		c.logger.Error("GetRolePermissions: ошибка при получении списка связей роли-привилегии", zap.Error(err))
		return utils.ErrorResponse(ctx, err)
	}

	return utils.SuccessResponse(
		ctx,
		rpList,
		"Список связей роли-привилегии успешно получен",
		http.StatusOK,
		rpTotal,
	)
}

func (c *RolePermissionController) FindRolePermission(ctx echo.Context) error {
	reqCtx := ctx.Request().Context()

	id, err := strconv.ParseUint(ctx.Param("id"), 10, 64)
	if err != nil {
		c.logger.Error("FindRolePermission: некорректный ID связи", zap.String("id", ctx.Param("id")), zap.Error(err))
		return utils.ErrorResponse(ctx, fmt.Errorf("некорректный ID связи: %w", apperrors.ErrBadRequest))
	}

	res, err := c.rpService.FindRolePermission(reqCtx, id)
	if err != nil {
		c.logger.Error("FindRolePermission: ошибка поиска связи", zap.Uint64("id", id), zap.Error(err))
		return utils.ErrorResponse(ctx, err)
	}

	return utils.SuccessResponse(
		ctx,
		res,
		"Связь роли-привилегии успешно найдена",
		http.StatusOK,
	)
}

func (c *RolePermissionController) CreateRolePermission(ctx echo.Context) error {
	reqCtx := ctx.Request().Context()

	var dto dto.CreateRolePermissionDTO
	if err := ctx.Bind(&dto); err != nil {
		c.logger.Error("CreateRolePermission: неверный запрос (bind)", zap.Error(err))
		return utils.ErrorResponse(ctx, fmt.Errorf("неверный формат запроса: %w", apperrors.ErrBadRequest))
	}

	if err := ctx.Validate(&dto); err != nil {
		c.logger.Error("CreateRolePermission: ошибка валидации данных", zap.Error(err))
		return utils.ErrorResponse(ctx, err)
	}

	res, err := c.rpService.CreateRolePermission(reqCtx, dto)
	if err != nil {
		c.logger.Error("CreateRolePermission: ошибка при создании связи", zap.Error(err))
		return utils.ErrorResponse(ctx, err)
	}

	return utils.SuccessResponse(
		ctx,
		res,
		"Связь роли-привилегии успешно создана",
		http.StatusCreated,
	)
}

func (c *RolePermissionController) UpdateRolePermission(ctx echo.Context) error {
	reqCtx := ctx.Request().Context()

	id, err := strconv.ParseUint(ctx.Param("id"), 10, 64)
	if err != nil {
		c.logger.Error("UpdateRolePermission: некорректный ID связи", zap.String("id", ctx.Param("id")), zap.Error(err))
		return utils.ErrorResponse(ctx, fmt.Errorf("некорректный ID связи: %w", apperrors.ErrBadRequest))
	}

	var dto dto.UpdateRolePermissionDTO
	if err := ctx.Bind(&dto); err != nil {
		c.logger.Error("UpdateRolePermission: неверный запрос (bind)", zap.Error(err))
		return utils.ErrorResponse(ctx, fmt.Errorf("неверный формат запроса: %w", apperrors.ErrBadRequest))
	}

	if err := ctx.Validate(&dto); err != nil {
		c.logger.Error("UpdateRolePermission: ошибка валидации данных", zap.Error(err))
		return utils.ErrorResponse(ctx, err)
	}

	res, err := c.rpService.UpdateRolePermission(reqCtx, id, dto)
	if err != nil {
		c.logger.Error("UpdateRolePermission: ошибка при обновлении связи", zap.Uint64("id", id), zap.Error(err))
		return utils.ErrorResponse(ctx, err)
	}

	return utils.SuccessResponse(
		ctx,
		res,
		"Связь роли-привилегии успешно обновлена",
		http.StatusOK,
	)
}

func (c *RolePermissionController) DeleteRolePermission(ctx echo.Context) error {
	reqCtx := ctx.Request().Context()

	id, err := strconv.ParseUint(ctx.Param("id"), 10, 64)
	if err != nil {
		c.logger.Error("DeleteRolePermission: некорректный ID связи", zap.String("id", ctx.Param("id")), zap.Error(err))
		return utils.ErrorResponse(ctx, fmt.Errorf("некорректный ID связи: %w", apperrors.ErrBadRequest))
	}

	err = c.rpService.DeleteRolePermission(reqCtx, id)
	if err != nil {
		c.logger.Error("DeleteRolePermission: ошибка при удалении связи", zap.Uint64("id", id), zap.Error(err))
		return utils.ErrorResponse(ctx, err)
	}

	return utils.SuccessResponse(
		ctx,
		struct{}{},
		"Связь роли-привилегии успешно удалена",
		http.StatusOK,
	)
}
