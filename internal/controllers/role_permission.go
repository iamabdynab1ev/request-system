package controllers

import (
	"fmt"
	"net/http"
	"strconv"

	"request-system/internal/dto"
	"request-system/internal/services"
	apperrors "request-system/pkg/errors"
	"request-system/pkg/utils"

	"go.uber.org/zap"

	"github.com/labstack/echo/v4"
)

type RolePermissionController struct {
	rpService *services.RolePermissionService
	logger    *zap.Logger
}

func NewRolePermissionController(
	rpService *services.RolePermissionService,
	logger *zap.Logger,
) *RolePermissionController {
	return &RolePermissionController{
		rpService: rpService,
		logger:    logger,
	}
}

func (c *RolePermissionController) GetRolePermissions(ctx echo.Context) error {
	reqCtx := ctx.Request().Context()

	res, err := c.rpService.GetRolePermissions(reqCtx, 6, 10)
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

func (c *RolePermissionController) FindRolePermission(ctx echo.Context) error {
	reqCtx := ctx.Request().Context()

	id, err := strconv.ParseUint(ctx.Param("id"), 10, 64)
	if err != nil {
		return utils.ErrorResponse(ctx, fmt.Errorf("invalid role_permission ID format: %w", apperrors.ErrBadRequest))
	}

	res, err := c.rpService.FindRolePermission(reqCtx, id)
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

func (c *RolePermissionController) CreateRolePermission(ctx echo.Context) error {
	reqCtx := ctx.Request().Context()

	var dto dto.CreateRolePermissionDTO
	if err := ctx.Bind(&dto); err != nil {
		c.logger.Error("неверный запрос", zap.Error(err))
		return utils.ErrorResponse(ctx, fmt.Errorf("request binding failed: %w", apperrors.ErrBadRequest))
	}

	if err := ctx.Validate(&dto); err != nil {
		c.logger.Error("Ощибка при валидации данных: ", zap.Error(err))
		return utils.ErrorResponse(ctx, err)
	}

	res, err := c.rpService.CreateRolePermission(reqCtx, dto)
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
		"Successfully created",
		http.StatusOK,
	)
}

func (c *RolePermissionController) UpdateRolePermission(ctx echo.Context) error {
	reqCtx := ctx.Request().Context()

	id, err := strconv.ParseUint(ctx.Param("id"), 10, 64)
	if err != nil {
		return utils.ErrorResponse(ctx, fmt.Errorf("invalid role_permission ID format in URL: %w", apperrors.ErrBadRequest))
	}

	var dto dto.UpdateRolePermissionDTO
	if err := ctx.Bind(&dto); err != nil {
		return utils.ErrorResponse(ctx, fmt.Errorf("request binding failed: %w", apperrors.ErrBadRequest))
	}

	if err := ctx.Validate(&dto); err != nil {
		return utils.ErrorResponse(ctx, err)
	}

	res, err := c.rpService.UpdateRolePermission(reqCtx, id, dto)
	if err != nil {
		return utils.ErrorResponse(
			ctx,
			err,
		)
	}

	return utils.SuccessResponse(
		ctx,
		res,
		"Successfully updated",
		http.StatusOK,
	)
}

func (c *RolePermissionController) DeleteRolePermission(ctx echo.Context) error {
	reqCtx := ctx.Request().Context()

	id, err := strconv.ParseUint(ctx.Param("id"), 10, 64)
	if err != nil {
		return utils.ErrorResponse(ctx, fmt.Errorf("invalid role_permission ID format: %w", apperrors.ErrBadRequest))
	}

	err = c.rpService.DeleteRolePermission(reqCtx, id)
	if err != nil {
		return utils.ErrorResponse(
			ctx,
			err,
		)
	}

	return utils.SuccessResponse(
		ctx,
		struct{}{},
		"Successfully deleted",
		http.StatusOK,
	)
}
