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
	permissionService services.PermissionServiceInterface
	logger            *zap.Logger
}

func NewPermissionController(permissionService services.PermissionServiceInterface, logger *zap.Logger) *PermissionController {
	return &PermissionController{
		permissionService: permissionService,
		logger:            logger,
	}
}

func (c *PermissionController) GetPermissions(ctx echo.Context) error {
	reqCtx := ctx.Request().Context()
	limit, offset, _ := utils.ParsePaginationParams(ctx.QueryParams())

	// 1. Сервис теперь возвращает ОДИН объект с данными и пагинацией внутри.
	paginatedResponse, err := c.permissionService.GetPermissions(reqCtx, limit, offset)
	if err != nil {
		c.logger.Error("Ошибка в контроллере при получении списка привилегий", zap.Error(err))
		return utils.ErrorResponse(ctx, err)
	}

	return utils.SuccessResponse(
		ctx,
		paginatedResponse,
		"Список привилегий успешно получен",
		http.StatusOK,
	)
}

func (c *PermissionController) FindPermission(ctx echo.Context) error {
	id, err := strconv.ParseUint(ctx.Param("id"), 10, 64)
	if err != nil {
		return utils.ErrorResponse(ctx, apperrors.ErrBadRequest)
	}
	res, err := c.permissionService.FindPermissionByID(ctx.Request().Context(), id)
	if err != nil {
		return utils.ErrorResponse(ctx, err)
	}
	return utils.SuccessResponse(ctx, res, "Successfully", http.StatusOK)
}

func (c *PermissionController) CreatePermission(ctx echo.Context) error {
	var dto dto.CreatePermissionDTO
	if err := ctx.Bind(&dto); err != nil {
		return utils.ErrorResponse(ctx, apperrors.ErrBadRequest)
	}
	if err := ctx.Validate(&dto); err != nil {
		return utils.ErrorResponse(ctx, err)
	}
	res, err := c.permissionService.CreatePermission(ctx.Request().Context(), dto)
	if err != nil {
		return utils.ErrorResponse(ctx, err)
	}
	return utils.SuccessResponse(ctx, res, "Successfully", http.StatusCreated)
}

func (c *PermissionController) UpdatePermission(ctx echo.Context) error {
	id, err := strconv.ParseUint(ctx.Param("id"), 10, 64)
	if err != nil {
		return utils.ErrorResponse(ctx, apperrors.ErrBadRequest)
	}
	var dto dto.UpdatePermissionDTO
	if err = ctx.Bind(&dto); err != nil {
		return utils.ErrorResponse(ctx, apperrors.ErrBadRequest)
	}
	if err = ctx.Validate(&dto); err != nil {
		return utils.ErrorResponse(ctx, err)
	}
	res, err := c.permissionService.UpdatePermission(ctx.Request().Context(), id, dto)
	if err != nil {
		return utils.ErrorResponse(ctx, err)
	}
	return utils.SuccessResponse(ctx, res, "Successfully", http.StatusOK)
}

func (c *PermissionController) DeletePermission(ctx echo.Context) error {
	id, err := strconv.ParseUint(ctx.Param("id"), 10, 64)
	if err != nil {
		return utils.ErrorResponse(ctx, apperrors.ErrBadRequest)
	}
	err = c.permissionService.DeletePermission(ctx.Request().Context(), id)
	if err != nil {
		return utils.ErrorResponse(ctx, err)
	}
	return ctx.NoContent(http.StatusNoContent)
}
