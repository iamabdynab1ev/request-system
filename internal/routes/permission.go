// routes/permission.go
package routes

import (
	"request-system/internal/authz"
	"request-system/internal/controllers"
	"request-system/internal/services"
	"request-system/pkg/middleware"

	"github.com/labstack/echo/v4"
	"go.uber.org/zap"
)

func runPermissionRouter(
	secureGroup *echo.Group,
	permissionService services.PermissionServiceInterface, // <-- Принимаем готовый сервис
	logger *zap.Logger,
	authMW *middleware.AuthMiddleware,
) {
	permCtrl := controllers.NewPermissionController(permissionService, logger)

	perms := secureGroup.Group("/permission")

	perms.GET("", permCtrl.GetPermissions, authMW.AuthorizeAny(authz.PermissionsView))
	perms.GET("/:id", permCtrl.FindPermission, authMW.AuthorizeAny(authz.PermissionsView))
	perms.POST("", permCtrl.CreatePermission, authMW.AuthorizeAny(authz.PermissionsCreate))
	perms.PUT("/:id", permCtrl.UpdatePermission, authMW.AuthorizeAny(authz.PermissionsUpdate))
	perms.DELETE("/:id", permCtrl.DeletePermission, authMW.AuthorizeAny(authz.PermissionsDelete))
}
