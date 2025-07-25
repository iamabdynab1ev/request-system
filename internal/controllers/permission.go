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

type PermissionController struct {
	permService services.PermissionServiceInterface
	logger      *zap.Logger
}

func NewPermissionController(permService services.PermissionServiceInterface, logger *zap.Logger) *PermissionController {
	return &PermissionController{
		permService: permService,
		logger:      logger,
	}
}

func (c *PermissionController) GetPermissions(ctx echo.Context) error {
	reqCtx := ctx.Request().Context()
	filter := utils.ParseFilterFromQuery(ctx.Request().URL.Query())

	res, err := c.permService.GetPermissions(reqCtx, uint64(filter.Limit), uint64(filter.Offset))
	if err != nil {
		c.logger.Error("Ошибка получения списка привилегий", zap.Error(err))
		return utils.ErrorResponse(ctx, err)
	}

	total := res.Pagination.TotalCount

	return utils.SuccessResponse(ctx, res, "Список привилегий успешно получен", http.StatusOK, total)
}

// FindPermissionByID: Получение одной привилегии по ID.
func (c *PermissionController) FindPermission(ctx echo.Context) error {
	reqCtx := ctx.Request().Context()
	id, err := strconv.ParseUint(ctx.Param("id"), 10, 64)
	if err != nil {
		return utils.ErrorResponse(ctx, apperrors.ErrBadRequest)
	}

	res, err := c.permService.FindPermissionByID(reqCtx, id)
	if err != nil {
		c.logger.Error("Ошибка поиска привилегии", zap.Error(err), zap.Uint64("permID", id))
		return utils.ErrorResponse(ctx, err)
	}
	return utils.SuccessResponse(ctx, res, "Привилегия успешно найдена", http.StatusOK)
}

// CreatePermission: Создание новой привилегии.
func (c *PermissionController) CreatePermission(ctx echo.Context) error {
	reqCtx := ctx.Request().Context()
	var dto dto.CreatePermissionDTO
	if err := ctx.Bind(&dto); err != nil {
		return utils.ErrorResponse(ctx, apperrors.ErrBadRequest)
	}
	if err := ctx.Validate(&dto); err != nil {
		return utils.ErrorResponse(ctx, err)
	}

	res, err := c.permService.CreatePermission(reqCtx, dto)
	if err != nil {
		c.logger.Error("Ошибка при создании привилегии", zap.Error(err))
		return utils.ErrorResponse(ctx, err)
	}
	return utils.SuccessResponse(ctx, res, "Привилегия успешно создана", http.StatusCreated)
}

// UpdatePermission: Обновление привилегии.
func (c *PermissionController) UpdatePermission(ctx echo.Context) error {
	reqCtx := ctx.Request().Context()
	id, err := strconv.ParseUint(ctx.Param("id"), 10, 64)
	if err != nil {
		return utils.ErrorResponse(ctx, apperrors.ErrBadRequest)
	}
	var dto dto.UpdatePermissionDTO
	if err := ctx.Bind(&dto); err != nil {
		return utils.ErrorResponse(ctx, apperrors.ErrBadRequest)
	}
	if err := ctx.Validate(&dto); err != nil {
		return utils.ErrorResponse(ctx, err)
	}

	res, err := c.permService.UpdatePermission(reqCtx, id, dto)
	if err != nil {
		c.logger.Error("Ошибка при обновлении привилегии", zap.Uint64("permID", id), zap.Error(err))
		return utils.ErrorResponse(ctx, err)
	}
	return utils.SuccessResponse(ctx, res, "Привилегия успешно обновлена", http.StatusOK)
}

// DeletePermission: Удаление привилегии.
func (c *PermissionController) DeletePermission(ctx echo.Context) error {
	reqCtx := ctx.Request().Context()
	id, err := strconv.ParseUint(ctx.Param("id"), 10, 64)
	if err != nil {
		return utils.ErrorResponse(ctx, apperrors.ErrBadRequest)
	}

	err = c.permService.DeletePermission(reqCtx, id)
	if err != nil {
		c.logger.Error("Ошибка при удалении привилегии", zap.Uint64("permID", id), zap.Error(err))
		return utils.ErrorResponse(ctx, err)
	}
	return utils.SuccessResponse(ctx, struct{}{}, "Привилегия успешно удалена", http.StatusOK)
}
