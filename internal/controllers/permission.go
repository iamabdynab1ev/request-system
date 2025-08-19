// controllers/permission.go
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
	return &PermissionController{permService: permService, logger: logger}
}

func (c *PermissionController) GetPermissions(ctx echo.Context) error {
	reqCtx := ctx.Request().Context()
	filter := utils.ParseFilterFromQuery(ctx.Request().URL.Query())
	search := ctx.QueryParam("search")

	res, err := c.permService.GetPermissions(reqCtx, uint64(filter.Limit), uint64(filter.Offset), search)
	if err != nil {
		c.logger.Error("Ошибка получения списка привилегий", zap.Error(err))
		return utils.ErrorResponse(ctx, err)
	}

	return utils.SuccessResponse(ctx, res.List, "Список привилегий успешно получен", http.StatusOK, res.Pagination.TotalCount)
}
func (c *PermissionController) FindPermission(ctx echo.Context) error {
	id, err := strconv.ParseUint(ctx.Param("id"), 10, 64)
	if err != nil {
		return utils.ErrorResponse(ctx, apperrors.NewHttpError(http.StatusBadRequest, "Неверный ID привилегии", err))
	}

	res, err := c.permService.FindPermissionByID(ctx.Request().Context(), id)
	if err != nil {
		return utils.ErrorResponse(ctx, err)
	}
	return utils.SuccessResponse(ctx, res, "Привилегия успешно найдена", http.StatusOK)
}

func (c *PermissionController) CreatePermission(ctx echo.Context) error {
	var dto dto.CreatePermissionDTO
	if err := ctx.Bind(&dto); err != nil {
		return utils.ErrorResponse(ctx, apperrors.NewHttpError(http.StatusBadRequest, "Неверный формат запроса", err))
	}
	if err := ctx.Validate(&dto); err != nil {
		return utils.ErrorResponse(ctx, err)
	}

	res, err := c.permService.CreatePermission(ctx.Request().Context(), dto)
	if err != nil {
		return utils.ErrorResponse(ctx, err)
	}
	return utils.SuccessResponse(ctx, res, "Привилегия успешно создана", http.StatusCreated)
}

func (c *PermissionController) UpdatePermission(ctx echo.Context) error {
	id, err := strconv.ParseUint(ctx.Param("id"), 10, 64)
	if err != nil {
		return utils.ErrorResponse(ctx, apperrors.NewHttpError(http.StatusBadRequest, "Неверный ID привилегии", err))
	}

	var dto dto.UpdatePermissionDTO
	if err := ctx.Bind(&dto); err != nil {
		return utils.ErrorResponse(ctx, apperrors.NewHttpError(http.StatusBadRequest, "Неверный формат запроса", err))
	}
	if err := ctx.Validate(&dto); err != nil {
		return utils.ErrorResponse(ctx, err)
	}

	res, err := c.permService.UpdatePermission(ctx.Request().Context(), id, dto)
	if err != nil {
		return utils.ErrorResponse(ctx, err)
	}
	return utils.SuccessResponse(ctx, res, "Привилегия успешно обновлена", http.StatusOK)
}

func (c *PermissionController) DeletePermission(ctx echo.Context) error {
	id, err := strconv.ParseUint(ctx.Param("id"), 10, 64)
	if err != nil {
		return utils.ErrorResponse(ctx, apperrors.NewHttpError(http.StatusBadRequest, "Неверный ID привилегии", err))
	}

	err = c.permService.DeletePermission(ctx.Request().Context(), id)
	if err != nil {
		return utils.ErrorResponse(ctx, err)
	}
	return utils.SuccessResponse(ctx, struct{}{}, "Привилегия успешно удалена", http.StatusOK)
}
