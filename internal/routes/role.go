package routes

import (
	"request-system/internal/controllers"
	"request-system/internal/repositories"
	"request-system/internal/services"
	"github.com/jackc/pgx/v5/pgxpool"
	"github.com/labstack/echo/v4"
)

func RUN_ROLE_ROUTER(e *echo.Echo, dbConn *pgxpool.Pool) {
	var (
		roleRepository = repositories.NewRoleRepository(dbConn)
		roleService    = services.NewRoleService(roleRepository) 
		roleCtrl       = controllers.NewRoleController(roleService)
	)

	e.GET("/role", roleCtrl.GetRoles)
	e.GET("/role/:id", roleCtrl.FindRole)
	e.POST("/role", roleCtrl.CreateRole)
	e.PUT("/role/:id", roleCtrl.UpdateRole)
	e.DELETE("/role/:id", roleCtrl.DeleteRole)
}