package routes

import (
	"request-system/internal/controllers"
	"request-system/internal/repositories"
	"request-system/internal/services"
	"github.com/jackc/pgx/v5/pgxpool"
	"github.com/labstack/echo/v4"
	"request-system/pkg/logger"
	
)

func RUN_PERMISSION_ROUTER(e *echo.Echo, dbConn *pgxpool.Pool) {
	var (
		logger = logger.NewLogger()

		permissionRepository = repositories.NewPermissionRepository(dbConn)
		permissionService    = services.NewPermissionService(permissionRepository, logger)
		permissionCtrl       = controllers.NewPermissionController(permissionService, logger)
	)
	e.GET("/permission", permissionCtrl.GetPermissions)
	e.GET("/permission/:id", permissionCtrl.FindPermission)
	e.POST("/permission", permissionCtrl.CreatePermission)
	e.PUT("/permission/:id", permissionCtrl.UpdatePermission)
	e.DELETE("/permission/:id", permissionCtrl.DeletePermission)
}