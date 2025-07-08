package routes

import (
	"request-system/internal/controllers"
	"request-system/internal/repositories"
	"request-system/internal/services"

	"github.com/jackc/pgx/v5/pgxpool"
	"github.com/labstack/echo/v4"
	"go.uber.org/zap"
)

func runPermissionRouter(secureGroup *echo.Group, dbConn *pgxpool.Pool, logger *zap.Logger) {
	var (
		permissionRepository = repositories.NewPermissionRepository(dbConn)
		permissionService    = services.NewPermissionService(permissionRepository, logger)
		permissionCtrl       = controllers.NewPermissionController(permissionService, logger)
	)
	secureGroup.GET("/permission", permissionCtrl.GetPermissions)
	secureGroup.GET("/permission/:id", permissionCtrl.FindPermission)
	secureGroup.POST("/permission", permissionCtrl.CreatePermission)
	secureGroup.PUT("/permission/:id", permissionCtrl.UpdatePermission)
	secureGroup.DELETE("/permission/:id", permissionCtrl.DeletePermission)
}
