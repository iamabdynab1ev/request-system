package routes

import (
	"request-system/internal/controllers"
	"github.com/labstack/echo/v4"
)

var permissionCtrl = controllers.NewPermissionController()

func RUN_PERMISSION_ROUTER(e *echo.Echo) {
	e.GET("permission", permissionCtrl.GetPermissions)
	e.GET("permission/:id", permissionCtrl.FindPermissions)
	e.POST("permission", permissionCtrl.CreatePermissions)
	e.PUT("permission/:id", permissionCtrl.UpdatePermissions)
	e.DELETE("permission/:id", permissionCtrl.DeletePermissions)
}