// routes/permission.go
package routes

import (
	"request-system/internal/authz"
	"request-system/internal/controllers"
	"request-system/internal/repositories"
	"request-system/internal/services"
	"request-system/pkg/middleware"

	"github.com/jackc/pgx/v5/pgxpool"
	"github.com/labstack/echo/v4"
	"go.uber.org/zap"
)

func runPermissionRouter(
	secureGroup *echo.Group,
	dbConn *pgxpool.Pool,
	logger *zap.Logger,
	authMW *middleware.AuthMiddleware,
) {
	permRepo := repositories.NewPermissionRepository(dbConn, logger)
	userRepo := repositories.NewUserRepository(dbConn, logger)
	permService := services.NewPermissionService(permRepo, userRepo, logger)
	permCtrl := controllers.NewPermissionController(permService, logger)

	perms := secureGroup.Group("/permission")

	perms.GET("", permCtrl.GetPermissions, authMW.AuthorizeAny(authz.PermissionsView))
	perms.GET("/:id", permCtrl.FindPermission, authMW.AuthorizeAny(authz.PermissionsView))
	perms.POST("", permCtrl.CreatePermission)
	perms.PUT("/:id", permCtrl.UpdatePermission)
	perms.DELETE("/:id", permCtrl.DeletePermission)
}
