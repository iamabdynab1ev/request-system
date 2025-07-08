package routes

import (
	"request-system/internal/controllers"
	"request-system/internal/repositories"
	"request-system/internal/services"
	"github.com/jackc/pgx/v5/pgxpool"
	"request-system/pkg/logger"
	"github.com/labstack/echo/v4"
)

func runRolePermissionRouter(e *echo.Echo, dbConn *pgxpool.Pool) {
	var (
		logger = logger.NewLogger()

		rpRepository = repositories.NewRolePermissionRepository(dbConn)
		rpService    = services.NewRolePermissionService(rpRepository, logger)
		rpCtrl       = controllers.NewRolePermissionController(rpService, logger)
	)

	e.GET("/role-permission", rpCtrl.GetRolePermissions)
	e.GET("/role-permission/:id", rpCtrl.FindRolePermission)
	e.POST("/role-permission", rpCtrl.CreateRolePermission)
	e.PUT("/role-permission/:id", rpCtrl.UpdateRolePermission)
	e.DELETE("/role-permission/:id", rpCtrl.DeleteRolePermission)
}