package routes

import (
	"request-system/internal/controllers"
	"github.com/labstack/echo/v4"
)

var roleCtrl = controllers.NewRoleController()

func RUN_ROLE_ROUTER(e *echo.Echo) {
	e.GET("role", roleCtrl.GetRoles)
	e.GET("role/:id", roleCtrl.FindRoles)
	e.POST("role", roleCtrl.CreateRoles)
	e.PUT("role/:id", roleCtrl.UpdateRoles)
	e.DELETE("role/:id", roleCtrl.DeleteRoles)
}