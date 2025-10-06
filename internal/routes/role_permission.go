package routes

import (
	"github.com/labstack/echo/v4"
	"go.uber.org/zap"

	"request-system/internal/authz"
	"request-system/internal/controllers"
	"request-system/internal/services"
	"request-system/pkg/middleware"
)

func runRolePermissionRouter(
	secureGroup *echo.Group,
	rpService services.RolePermissionServiceInterface,
	logger *zap.Logger,
	authMW *middleware.AuthMiddleware,
) {
	rpCtrl := controllers.NewRolePermissionController(rpService, logger)

	rpGroup := secureGroup.Group("/role-permission")

	rpGroup.GET("", rpCtrl.GetRolePermissions, authMW.AuthorizeAny(authz.RolesView))
	rpGroup.GET("/:id", rpCtrl.FindRolePermission, authMW.AuthorizeAny(authz.RolesView))
	rpGroup.POST("", rpCtrl.CreateRolePermission, authMW.AuthorizeAny(authz.RolesUpdate))
	rpGroup.PUT("/:id", rpCtrl.UpdateRolePermission, authMW.AuthorizeAny(authz.RolesUpdate))
	rpGroup.DELETE("/:id", rpCtrl.DeleteRolePermission, authMW.AuthorizeAny(authz.RolesUpdate))
}
