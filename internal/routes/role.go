package routes

import (
	"request-system/internal/controllers"
	"request-system/internal/repositories"
	"request-system/internal/services"
	"request-system/pkg/logger"

	"github.com/jackc/pgx/v5/pgxpool"
	"github.com/labstack/echo/v4"
)

func RUN_ROLE_ROUTER(e *echo.Echo, dbConn *pgxpool.Pool) {
	var (
		logger =logger.NewLogger()

		roleRepository = repositories.NewRoleRepository(dbConn)
		roleService    = services.NewRoleService(roleRepository, logger) 
		roleCtrl       = controllers.NewRoleController(roleService, logger)
	)

	e.GET("/role", roleCtrl.GetRoles)
	e.GET("/role/:id", roleCtrl.FindRole)
	e.POST("/role", roleCtrl.CreateRole)
	e.PUT("/role/:id", roleCtrl.UpdateRole)
	e.DELETE("/role/:id", roleCtrl.DeleteRole)
}