package controllers
import (
	"net/http"

	"request-system/pkg/utils"
	"github.com/labstack/echo/v4"
)

type PermissionController struct {}

func NewPermissionController() *PermissionController {
	return &PermissionController{}
}


func (c *PermissionController) GetPermissions(ctx echo.Context) error {
	return ctx.JSON(http.StatusOK, utils.SuccessResponse("success") )
}


func (c *PermissionController) FindPermissions(ctx echo.Context) error {
	return ctx.JSON(http.StatusOK, utils.SuccessResponse("success") )
}


func (c *PermissionController) CreatePermissions(ctx echo.Context) error {
	return ctx.JSON(http.StatusOK, utils.SuccessResponse("success") )
}


func (c *PermissionController) UpdatePermissions(ctx echo.Context) error {
	return ctx.JSON(http.StatusOK, utils.SuccessResponse("success") )
}

func (c *PermissionController) DeletePermissions(ctx echo.Context) error {
	return ctx.JSON(http.StatusOK, utils.SuccessResponse("success") )
}


