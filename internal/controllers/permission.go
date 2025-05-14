package controllers

import (
	"net/http"

	"request-system/pkg/utils"

	"github.com/labstack/echo/v4"
)

type PermissionController struct{}

func NewPermissionController() *PermissionController {
	return &PermissionController{}
}

func (c *PermissionController) GetPermissions(ctx echo.Context) error {
	return utils.SuccessResponse(
		ctx,
		struct{}{},
		"Successfully",
		http.StatusOK,
	)
}

func (c *PermissionController) FindPermissions(ctx echo.Context) error {
	return utils.SuccessResponse(
		ctx,
		struct{}{},
		"Successfully",
		http.StatusOK,
	)
}

func (c *PermissionController) CreatePermissions(ctx echo.Context) error {
	return utils.SuccessResponse(
		ctx,
		struct{}{},
		"Successfully",
		http.StatusOK,
	)
}

func (c *PermissionController) UpdatePermissions(ctx echo.Context) error {
	return utils.SuccessResponse(
		ctx,
		struct{}{},
		"Successfully",
		http.StatusOK,
	)
}

func (c *PermissionController) DeletePermissions(ctx echo.Context) error {
	return utils.SuccessResponse(
		ctx,
		struct{}{},
		"Successfully",
		http.StatusOK,
	)
}
