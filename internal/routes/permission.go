package routes

import (
	"request-system/internal/controllers"
	"request-system/internal/repositories"
	"request-system/internal/services" // <-- ДОБАВЛЕНО для AuthPermissionServiceInterface
	"request-system/pkg/middleware"    // <-- ДОБАВЛЕНО для AuthMiddleware

	"github.com/jackc/pgx/v5/pgxpool"
	"github.com/labstack/echo/v4"
	"go.uber.org/zap"
)

func runPermissionRouter(secureGroup *echo.Group, dbConn *pgxpool.Pool, logger *zap.Logger, authMW *middleware.AuthMiddleware, authPermissionService services.AuthPermissionServiceInterface) { // <-- ИЗМЕНЕНО
	var (
		permissionRepository = repositories.NewPermissionRepository(dbConn, logger)
		permissionService    = services.NewPermissionService(permissionRepository, logger)
		permissionCtrl       = controllers.NewPermissionController(permissionService, logger)
	)
	secureGroup.GET("/permission", permissionCtrl.GetPermissions, authMW.AuthorizeAny("permissions:view:all", "permissions:manage"))
	secureGroup.GET("/permission/:id", permissionCtrl.FindPermission, authMW.AuthorizeAny("permissions:view:all", "permissions:manage"))
	secureGroup.POST("/permission", permissionCtrl.CreatePermission, authMW.AuthorizeAny("permissions:create", "permissions:manage"))
	secureGroup.PUT("/permission/:id", permissionCtrl.UpdatePermission, authMW.AuthorizeAny("permissions:update", "permissions:manage"))
	secureGroup.DELETE("/permission/:id", permissionCtrl.DeletePermission, authMW.AuthorizeAny("permissions:delete", "permissions:manage"))
}
