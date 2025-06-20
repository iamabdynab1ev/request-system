package controllers

import (
	"net/http"
	"strconv"

	"request-system/internal/dto"
	"request-system/internal/services"
	"request-system/pkg/utils"

	"go.uber.org/zap"

	"github.com/labstack/echo/v4"
)

type PermissionController struct {
	permissionService *services.PermissionService
	logger            *zap.Logger
}

func NewPermissionController(
		permissionService *services.PermissionService,
		logger *zap.Logger,
) *PermissionController {
	return &PermissionController{
		permissionService: permissionService,
		logger: logger,
	}
}

func (c *PermissionController) GetPermissions(ctx echo.Context) error {
	reqCtx := ctx.Request().Context()

	res, err := c.permissionService.GetPermissions(reqCtx, 6, 10)
	if err != nil {
		return utils.ErrorResponse(
			ctx,
			err,
		)
	}

	return utils.SuccessResponse(
		ctx,
		res,
		"Successfully",
		http.StatusOK,
	)
}


func (c *PermissionController) FindPermission(ctx echo.Context) error {
	reqCtx := ctx.Request().Context()

	id, err := strconv.ParseUint(ctx.Param("id"), 10, 64)
	if err != nil {
		return utils.ErrorResponse(ctx, err)
	}

	res, err := c.permissionService.FindPermission(reqCtx, id)
	if err != nil {
		return utils.ErrorResponse(
			ctx,
			err,
		)
	}

	return utils.SuccessResponse(
		ctx,
		res,
		"Successfully",
		http.StatusOK,
	)
}

func (c *PermissionController) CreatePermission(ctx echo.Context) error {
	reqCtx := ctx.Request().Context()

	var dto dto.CreatePermissionDTO
	if err := ctx.Bind(&dto); err != nil {
		c.logger.Error("неверный запрос", zap.Error(err))
		return utils.ErrorResponse(ctx, err)
	}

	if err := ctx.Validate(&dto); err != nil {
		c.logger.Error("Ощибка при валидации данных: ", zap.Error(err))
		return utils.ErrorResponse(ctx, err)
	}

	res, err := c.permissionService.CreatePermission(reqCtx, dto)
	if err != nil {
		c.logger.Error("Ощибка при создание: ", zap.Error(err))
		return utils.ErrorResponse(
			ctx,
			err,
		)
	}

	return utils.SuccessResponse(
		ctx,
		res,
		"Successfully",
		http.StatusOK,
	)
}

func (c *PermissionController) UpdatePermission(ctx echo.Context) error {
	reqCtx := ctx.Request().Context()

	id, err := strconv.ParseUint(ctx.Param("id"), 10, 64)
	if err != nil {
		return utils.ErrorResponse(ctx, err)
	}

	var dto dto.UpdatePermissionDTO
	if err := ctx.Bind(&dto); err != nil {
		return utils.ErrorResponse(ctx, err)
	}

	if err := ctx.Validate(&dto); err != nil {
		return utils.ErrorResponse(ctx, err)
	}

	res, err := c.permissionService.UpdatePermission(reqCtx, id, dto)
	if err != nil {
		return utils.ErrorResponse(
			ctx,
			err,
		)
	}

	return utils.SuccessResponse(
		ctx,
		res,
		"Successfully",
		http.StatusOK,
	)
}

func (c *PermissionController) DeletePermission(ctx echo.Context) error {
	reqCtx := ctx.Request().Context()

	id, err := strconv.ParseUint(ctx.Param("id"), 10, 64)
	if err != nil {
		return utils.ErrorResponse(ctx, err)
	}

	err = c.permissionService.DeletePermission(reqCtx, id)
	if err != nil {
		return utils.ErrorResponse(
			ctx,
			err,
		)
	}

	return utils.SuccessResponse(
		ctx,
		struct{}{},
		"Successfully",
		http.StatusOK,
	)
}